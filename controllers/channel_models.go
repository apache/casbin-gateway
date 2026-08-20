// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/proxy"
)

const maxModelsResponseBytes = 1024 * 1024

type fetchChannelModelsRequest struct {
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	BaseUrl  string `json:"baseUrl"`
	ApiKey   string `json:"apiKey"`
	AuthType string `json:"authType"`
}

// FetchChannelModels tries the small set of same-origin model endpoints used
// by Anthropic-compatible providers. A masked key is resolved only after the
// saved channel has passed the normal owner check.
func (c *ApiController) FetchChannelModels() {
	if c.RequireSignedIn() {
		return
	}
	var input fetchChannelModelsRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &input); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if input.Owner == "" {
		input.Owner = c.GetSessionUsername()
	}
	if !c.channelAccess(input.Owner) {
		c.ResponseError("unauthorized")
		return
	}
	if input.ApiKey == object.ApiKeyMask {
		if input.Name == "" {
			c.ResponseError("a saved channel is required when using the masked API key")
			return
		}
		stored, err := object.GetChannel(input.Owner + "/" + input.Name)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		if stored == nil {
			c.ResponseError("the channel does not exist")
			return
		}
		input.ApiKey = stored.ApiKey
	}
	if input.AuthType == "" {
		input.AuthType = "bearer"
	}
	if input.AuthType != "bearer" && input.AuthType != "x-api-key" {
		c.ResponseError("invalid channel auth type: " + input.AuthType)
		return
	}
	if models, ok := object.ProviderModels(input.Provider); ok {
		c.ResponseOk(models)
		return
	}

	candidates, err := object.BuildModelEndpointCandidates(input.BaseUrl, input.Provider)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	client := &http.Client{
		Timeout:       10 * time.Second,
		Transport:     proxy.Transport(),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	lastError := "no model endpoint succeeded"
	modelAuthType := object.ModelEndpointAuthType(input.Provider, input.AuthType)
	for _, candidate := range candidates {
		models, err := fetchModelsCandidate(client, candidate, input.ApiKey, modelAuthType)
		if err != nil {
			lastError = err.Error()
			continue
		}
		c.ResponseOk(models)
		return
	}
	c.ResponseError(lastError)
}

func fetchModelsCandidate(client *http.Client, endpoint, apiKey, authType string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if authType == "x-api-key" {
		req.Header.Set("x-api-key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("model request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("model endpoint returned %s", resp.Status)
	}
	limited := io.LimitReader(resp.Body, maxModelsResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read model response: %w", err)
	}
	if len(data) > maxModelsResponseBytes {
		return nil, fmt.Errorf("model response is too large")
	}

	var payload struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid model response")
	}
	models := payload.Models
	for _, item := range payload.Data {
		models = append(models, item.Id)
	}
	unique := map[string]bool{}
	result := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" && !unique[model] {
			unique[model] = true
			result = append(result, model)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("model response contains no models")
	}
	sort.Strings(result)
	return result, nil
}
