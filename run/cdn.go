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

package run

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/storage"
	"github.com/apache/casbin-gateway/util"
)

func filterFiles(filenames []string, ext string, siteName string) []string {
	res := []string{}
	for _, filename := range filenames {
		if !strings.HasSuffix(filename, ext) {
			continue
		}

		if strings.HasPrefix(siteName, "casdoor") {
			if strings.Contains(filename, ".chunk.js") || strings.Contains(filename, ".chunk.css") {
				continue
			}
		}

		res = append(res, filename)
	}
	return res
}

// assetDirs tells where a web build keeps its hashed assets. Create React App
// splits them into "static/js" and "static/css", Vite puts both in "assets".
type assetDirs struct {
	root string
	js   string
	css  string
}

func getAssetDirs(buildDir string) (*assetDirs, error) {
	if util.FileExist(filepath.Join(buildDir, "static")) {
		return &assetDirs{root: "static", js: "static/js", css: "static/css"}, nil
	}

	if util.FileExist(filepath.Join(buildDir, "assets")) {
		return &assetDirs{root: "assets", js: "assets", css: "assets"}, nil
	}

	return nil, fmt.Errorf("getAssetDirs() error: no \"static\" or \"assets\" folder in buildDir = %s", buildDir)
}

func uploadFolder(provider storage.StorageProvider, buildDir string, root string, dir string, ext string, siteName string) (string, error) {
	domainUrl := ""

	urlRoot := "/" + root
	dirPath := filepath.Join(buildDir, filepath.FromSlash(dir))
	filenames, err := util.ListFiles(dirPath)
	if err != nil {
		return "", err
	}

	filteredFilenames := filterFiles(filenames, ext, siteName)
	for _, filename := range filteredFilenames {
		data, err := os.ReadFile(filepath.Join(dirPath, filename))
		if err != nil {
			return "", err
		}
		fileBuffer := bytes.NewBuffer(data)

		objectKey := fmt.Sprintf("%s/%s", dir, filename)
		fileUrl, err := provider.PutObject("Built-in-Untracked", "", objectKey, fileBuffer)
		if err != nil {
			return "", err
		}

		index := strings.Index(fileUrl, urlRoot)
		if index == -1 {
			return "", fmt.Errorf("uploadFolder() error, fileUrl should contain \"%s/\", fileUrl = %s", urlRoot, fileUrl)
		}

		domainUrl = fileUrl[:index+len(urlRoot)] + "/"
		fmt.Printf("uploadFolder(): [/%s] -> [%s]\n", objectKey, fileUrl)
	}

	return domainUrl, nil
}

func updateHtml(domainUrl string, buildDir string, root string) {
	htmlPath := filepath.Join(buildDir, "index.html")
	html := util.ReadStringFromPath(htmlPath)

	html = strings.Replace(html, fmt.Sprintf("\"/%s/", root), fmt.Sprintf("\"%s", domainUrl), -1)
	util.WriteStringToPath(html, htmlPath)

	fmt.Printf("updateHtml(): index.html content:\n%s\n%s\n%s\n", strings.Repeat("=", 80), html, strings.Repeat("=", 80))
}

func gitUploadCdn(providerName string, siteName string) error {
	if providerName == "" {
		return nil
	}

	fmt.Printf("gitUploadCdn(): [%s]\n", siteName)

	path := GetRepoPath(siteName)
	buildDir := filepath.Join(path, "web/build")

	dirs, err := getAssetDirs(buildDir)
	if err != nil {
		return err
	}

	provider, err := storage.GetStorageProvider(providerName)
	if err != nil {
		return err
	}

	var domainUrl string
	domainUrl, err = uploadFolder(provider, buildDir, dirs.root, dirs.js, "js", siteName)
	if err != nil {
		return err
	}

	_, err = uploadFolder(provider, buildDir, dirs.root, dirs.css, "css", siteName)
	if err != nil {
		return err
	}

	updateHtml(domainUrl, buildDir, dirs.root)
	return nil
}
