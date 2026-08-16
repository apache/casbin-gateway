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

package casdoor

import (
	_ "embed"
	"os"

	"github.com/apache/casbin-gateway/conf"
	"github.com/beego/beego"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

//go:embed token_jwt_key.pem
var JwtPublicKey string

func InitCasdoorConfig() {
	// Read through conf.GetConfigString so that every value can be overridden by
	// an environment variable of the same name (see conf.GetConfigString). This
	// lets docker-compose point the gateway at a bundled, self-hosted Casdoor
	// without editing conf/app.conf.
	casdoorEndpoint := conf.GetConfigString("casdoorEndpoint")
	clientId := conf.GetConfigString("clientId")
	clientSecret := conf.GetConfigString("clientSecret")
	casdoorOrganization := conf.GetConfigString("casdoorOrganization")
	casdoorApplication := conf.GetConfigString("casdoorApplication")

	casdoorsdk.InitConfig(casdoorEndpoint, clientId, clientSecret, getJwtPublicKey(), casdoorOrganization, casdoorApplication)
}

// getJwtPublicKey resolves the certificate used to verify Casdoor-issued JWTs.
// A self-hosted Casdoor signs tokens with its own certificate, so the embedded
// key (which matches the public demo at door.casdoor.com) will not validate
// them. Resolution order:
//  1. casdoorCertPath - a file containing the PEM certificate (docker mount).
//  2. casdoorJwtPublicKey - the PEM certificate inline.
//  3. the embedded token_jwt_key.pem (matches door.casdoor.com).
func getJwtPublicKey() string {
	if path := conf.GetConfigString("casdoorCertPath"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		} else {
			beego.Error("casdoorCertPath set but could not be read:", err)
		}
	}

	if key := conf.GetConfigString("casdoorJwtPublicKey"); key != "" {
		return key
	}

	return JwtPublicKey
}
