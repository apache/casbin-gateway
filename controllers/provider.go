// Copyright 2023 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"encoding/json"
	"strings"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego/utils/pagination"
)

func (c *ApiController) providerAccess(owner string) bool {
	user := c.GetSessionUser()
	return user != nil && (user.IsAdmin || owner == c.GetSessionUsername())
}

// getProviderOwnerAndName splits an "owner/name" id. util.GetOwnerAndNameFromId()
// panics on malformed input, and the id here comes straight from the query
// string, so it is validated before being passed on.
func getProviderOwnerAndName(id string) (string, string, bool) {
	tokens := strings.Split(id, "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return "", "", false
	}

	return tokens[0], tokens[1], true
}

// providerSortFields is a whitelist: the sort field is interpolated into the
// ORDER BY clause by object.GetSession(), so it must not be taken from the
// query string unchecked.
var providerSortFields = map[string]bool{
	"owner": true, "name": true, "createdTime": true, "updatedTime": true,
	"displayName": true, "type": true, "baseUrl": true, "priority": true, "status": true, "displayRank": true,
}

func getProviderSortField(sortField string) string {
	if providerSortFields[sortField] {
		return sortField
	}

	return ""
}

// GetProviders returns a paginated list of providers with tenant isolation.
func (c *ApiController) GetProviders() {
	if c.RequireSignedIn() {
		return
	}

	owner := c.Input().Get("owner")
	if !c.GetSessionUser().IsAdmin {
		// A non-admin may only ever see their own providers.
		owner = c.GetSessionUsername()
	} else if owner == "admin" {
		// The admin's own owner name means "no filter", i.e. every provider.
		owner = ""
	}

	limit := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	field := c.Input().Get("field")
	value := c.Input().Get("value")
	sortField := getProviderSortField(c.Input().Get("sortField"))
	sortOrder := c.Input().Get("sortOrder")

	if limit == "" || page == "" {
		providers, err := object.GetProviders(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		c.ResponseOk(object.GetMaskedProviders(providers))
		return
	}

	limitInt := util.ParseInt(limit)
	count, err := object.GetProviderCount(owner, field, value)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	paginator := pagination.SetPaginator(c.Ctx, limitInt, count)
	providers, err := object.GetPaginationProviders(owner, paginator.Offset(), limitInt, field, value, sortField, sortOrder)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedProviders(providers), paginator.Nums())
}

// GetProvider returns a single provider by owner/name id.
func (c *ApiController) GetProvider() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _, ok := getProviderOwnerAndName(id)
	if !ok {
		c.ResponseError("invalid provider ID: " + id)
		return
	}
	if !c.providerAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	provider, err := object.GetProvider(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if provider == nil {
		c.ResponseError("the provider does not exist")
		return
	}

	c.ResponseOk(object.GetMaskedProvider(provider))
}

// AddProvider creates a new provider.
func (c *ApiController) AddProvider() {
	if c.RequireSignedIn() {
		return
	}

	var provider object.Provider
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &provider); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if provider.Name == "" {
		c.ResponseError("the provider name cannot be empty")
		return
	}

	provider.Owner = c.GetSessionUsername()
	provider.DisplayRank = 0
	c.Data["json"] = wrapActionResponse(object.AddProvider(&provider))
	c.ServeJSON()
}

// ParseProviderLink reads a vendor's "add this provider" link and answers with
// what it would fill in, without storing anything: the link comes from a
// website, so the person adding it sees the values first.
func (c *ApiController) ParseProviderLink() {
	if c.RequireSignedIn() {
		return
	}

	var form struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	provider, err := object.ParseProviderLink(c.GetSessionUsername(), form.Link)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(provider)
}

// UpdateProvider updates an existing provider.
func (c *ApiController) UpdateProvider() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _, ok := getProviderOwnerAndName(id)
	if !ok {
		c.ResponseError("invalid provider ID: " + id)
		return
	}
	if !c.providerAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	var provider object.Provider
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &provider); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateProvider(id, &provider))
	c.ServeJSON()
}

// MoveProvider moves one card in the shared desktop display order.
func (c *ApiController) MoveProvider() {
	if c.RequireAdmin() {
		return
	}
	var form struct {
		Id   string `json:"id"`
		From *int   `json:"from"`
		To   *int   `json:"to"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if form.From == nil || form.To == nil {
		c.ResponseError("both provider positions are required")
		return
	}
	if err := object.MoveProvider(form.Id, *form.From, *form.To); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(*form.To)
}

// DeleteProvider deletes a provider.
func (c *ApiController) DeleteProvider() {
	if c.RequireSignedIn() {
		return
	}

	var provider object.Provider
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &provider); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if !c.providerAccess(provider.Owner) {
		c.ResponseError("unauthorized")
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteProvider(&provider))
	c.ServeJSON()
}

// GetProviderModels lists the models a provider's upstream reports. The provider
// comes from the request body rather than from the database: the new-provider
// form has nothing saved yet.
func (c *ApiController) GetProviderModels() {
	if c.RequireSignedIn() {
		return
	}

	var provider object.Provider
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &provider); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if provider.Owner == "" {
		provider.Owner = c.GetSessionUsername()
	}
	if !c.providerAccess(provider.Owner) {
		c.ResponseError("unauthorized")
		return
	}
	if err := resolveProviderApiKey(&provider); err != nil {
		c.ResponseError(err.Error())
		return
	}

	models, err := object.FetchProviderModels(&provider)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(models)
}

// quotaProviders are the providers whose vendor balances the caller may see: an
// admin sees every one, anybody else only their own.
func (c *ApiController) quotaProviders() ([]*object.Provider, error) {
	owner := c.GetSessionUsername()
	if c.GetSessionUser().IsAdmin {
		owner = ""
	}
	return object.GetProviders(owner)
}

// GetProviderQuotas returns the vendor balances that are already known. It asks
// no vendor anything, so a page can call it as often as it likes.
func (c *ApiController) GetProviderQuotas() {
	if c.RequireSignedIn() {
		return
	}

	providers, err := c.quotaProviders()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetProviderQuotas(providers))
}

// RefreshProviderQuotas asks the vendors what is left. Without an id every
// provider the caller can see is refreshed, and without force the ones whose
// last answer is still fresh are left alone.
func (c *ApiController) RefreshProviderQuotas() {
	if c.RequireSignedIn() {
		return
	}

	var request struct {
		Id    string `json:"id"`
		Force bool   `json:"force"`
	}
	if len(c.Ctx.Input.RequestBody) > 0 {
		if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
			c.ResponseError(err.Error())
			return
		}
	}

	if request.Id == "" {
		providers, err := c.quotaProviders()
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(object.RefreshProviderQuotas(providers, request.Force))
		return
	}

	owner, _, ok := getProviderOwnerAndName(request.Id)
	if !ok {
		c.ResponseError("invalid provider ID: " + request.Id)
		return
	}
	if !c.providerAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	provider, err := object.GetProvider(request.Id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if provider == nil {
		c.ResponseError("the provider does not exist")
		return
	}

	// A refresh of one provider is always somebody pressing a button, so it is
	// never answered from the cache.
	c.ResponseOk(object.RefreshProviderQuotas([]*object.Provider{provider}, true))
}

// resolveProviderApiKey fills in the key a probe has to be made with. The
// browser only ever sees the mask, so an untouched key field means the one the
// provider already has stored; a provider that is not saved yet has none.
func resolveProviderApiKey(provider *object.Provider) error {
	if provider.ApiKey != object.ApiKeyMask {
		return nil
	}

	provider.ApiKey = ""
	if _, _, ok := getProviderOwnerAndName(provider.GetId()); !ok {
		return nil
	}

	stored, err := object.GetProvider(provider.GetId())
	if err != nil {
		return err
	}
	if stored != nil {
		provider.ApiKey = stored.ApiKey
	}
	return nil
}

// TestProvider tests connectivity to an upstream provider. The provider comes
// from the request body so that a form can be checked before it is saved; a
// body carrying only an id falls back to the stored provider.
func (c *ApiController) TestProvider() {
	if c.RequireSignedIn() {
		return
	}

	var provider object.Provider
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &provider); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if provider.Owner == "" {
		provider.Owner = c.GetSessionUsername()
	}
	if !c.providerAccess(provider.Owner) {
		c.ResponseError("unauthorized")
		return
	}

	if provider.BaseUrl == "" {
		id := provider.GetId()
		if _, _, ok := getProviderOwnerAndName(id); !ok {
			c.ResponseError("invalid provider ID: " + id)
			return
		}
		stored, err := object.GetProvider(id)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		if stored == nil {
			c.ResponseError("the provider does not exist")
			return
		}
		provider = *stored
	} else if err := resolveProviderApiKey(&provider); err != nil {
		c.ResponseError(err.Error())
		return
	}

	success, statusCode, message := object.TestProviderConnectivity(&provider)
	c.ResponseOk(map[string]interface{}{"success": success, "statusCode": statusCode, "message": message})
}
