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

// Package beacon implements a GA4GH Beacon (http://ga4gh.org/#/beacon) backed
// by the Google Genomics Variants service search API.
package beacon

import (
	"encoding/xml"
	"errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/genomics/v1"
	"google.golang.org/appengine"
	"net/http"
	"strconv"
)

type beaconConfig struct {
	variantSetIds []string
}

var config = beaconConfig{
	variantSetIds: []string{"3049512673186936334"},
}

func init() {
	http.HandleFunc("/", handler)
}

func requestToSearch(r *http.Request) (*genomics.SearchVariantsRequest, string, error) {
	allele := r.FormValue("allele")
	search := &genomics.SearchVariantsRequest{
		ReferenceName: r.FormValue("chromosome"),
		VariantSetIds: config.variantSetIds,
	}

	coord, err := strconv.ParseInt(r.FormValue("coordinate"), 10, 64)
	if err != nil {
		return nil, "", err
	}

	// Start is inclusive, End is exclusive.  Search exactly for coordinate.
	search.Start = coord
	search.End = search.Start + 1

	if search.ReferenceName == "" {
		return nil, "", errors.New("missing reference name")
	}
	if allele == "" {
		return nil, "", errors.New("missing allele")
	}

	return search, allele, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	search, allele, err := requestToSearch(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := google.DefaultClient(c, genomics.GenomicsReadonlyScope)
	if err != nil {
		http.Error(w, "Invalid server configuration", http.StatusInternalServerError)
	}
	genomicsService, err := genomics.New(client)
	if err != nil {
		http.Error(w, "Invalid server configuration", http.StatusInternalServerError)
	}
	variantsService := genomics.NewVariantsService(genomicsService)

	type beaconResponse struct {
		XMLName struct{} `xml:"BEACONResponse"`
		Exists  bool     `xml:"exists"`
	}
	var resp beaconResponse

	for {
		searchResponse, err := variantsService.Search(search).Do()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, variant := range searchResponse.Variants {
			if search.Start != variant.Start {
				continue
			}
			if allele == variant.ReferenceBases {
				resp.Exists = true
			} else {
				for _, base := range variant.AlternateBases {
					if base == allele {
						resp.Exists = true
						break
					}
				}
			}
		}

		if resp.Exists || searchResponse.NextPageToken == "" {
			break
		}
		search.PageToken = searchResponse.NextPageToken
	}

	w.Header().Set("Content-Type", "application/xml")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err = enc.Encode(resp); err != nil {
		http.Error(w, "Failed writing response", http.StatusInternalServerError)
	}
}
