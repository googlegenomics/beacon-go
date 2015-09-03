/*
 * Copyright (C) 2015 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 */

package beacon

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type variantResult struct {
	ReferenceName  string
	Start          string
	AlternateBases []string
}

type variantSearchResults struct {
	Variants []variantResult
}

type variantSearch struct {
	VariantSetIds []string `json:"variantSetIds"`
	ReferenceName string   `json:"referenceName"`
	Start         string   `json:"start"`
	End           string   `json:"end"`

	Allele string `json:"-"`
}

type beaconResponse struct {
	XMLName struct{} `xml:"BEACONResponse"`
	Exists  bool     `xml:"exists"`
}

func init() {
	http.HandleFunc("/", handler)
}

var keyCache = struct {
	sync.RWMutex
	cache map[string]string
}{cache: make(map[string]string)}

func getAPIKey(keyFileName string) (string, error) {
	keyCache.RLock()
	res, ok := keyCache.cache[keyFileName]
	keyCache.RUnlock()
	if ok {
		return res, nil
	}

	rawKey, err := ioutil.ReadFile(keyFileName)
	key := strings.TrimSpace(string(rawKey))
	if err != nil || key == ""{
		return "", err
	}

	keyCache.Lock()
	keyCache.cache[keyFileName] = key
	keyCache.Unlock()

	return key, nil
}

func QueryDataSource(
	c context.Context,
	source DataSource,
	search *variantSearch,
	reply chan<- bool) error {
	search.VariantSetIds = source.VariantSetIds

	jsonStr, err := json.Marshal(search)
	if err != nil {
		return err
	}

	key, err := getAPIKey(source.APIKeyFileName)
	if err != nil {
		return err
	}
	url := source.URL + "key=" + key

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := urlfetch.Client(c)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var results variantSearchResults
	err = json.Unmarshal(body, &results)
	if err != nil {
		return err
	}

	for _, variant := range results.Variants {
		if variant.ReferenceName == search.ReferenceName &&
			variant.Start == search.Start {
			for _, alternateBase := range variant.AlternateBases {
				if alternateBase == search.Allele {
					reply <- true
					return nil
				}
			}
		}
	}
	reply <- false
	return nil
}

func requestToSearch(r *http.Request) (*variantSearch, error) {
	search := &variantSearch{
		ReferenceName: r.FormValue("chromosome"),
		Start:         r.FormValue("coordinate"),
		Allele:        r.FormValue("allele"),
	}

	if search.ReferenceName == "" || search.Start == "" || search.Allele == "" {
		return search, errors.New("Bad parameters")
	}

	end, err := strconv.ParseInt(search.Start, 10, 64)
	if err != nil {
		return search, err
	}
	search.End = strconv.FormatInt(end+1, 10)

	return search, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	search, err := requestToSearch(r)
	if err != nil {
		http.Error(w, "Bad parameters", http.StatusInternalServerError)
		return
	}

	replies := make(chan bool)

	c := appengine.NewContext(r)

	var wg sync.WaitGroup
	wg.Add(len(DataSources))
	for _, source := range DataSources {
		go func(s DataSource) {
			err = QueryDataSource(c, s, search, replies)
			if err != nil {
				log.Errorf(c, "Failed to query (%s): %v", s.URL, err)
			}
			wg.Done()
		}(source)
	}
	go func() {
		wg.Wait()
		close(replies)
	}()

	resp := beaconResponse{Exists: false}
	for result := range replies {
		if result {
			resp.Exists = true
			break
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	err = enc.Encode(resp)
	if err != nil {
		http.Error(w, "Failed writing response", http.StatusInternalServerError)
	}
}
