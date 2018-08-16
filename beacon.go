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
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"cloud.google.com/go/bigquery"
	"google.golang.org/appengine"
)

type beaconConfig struct {
	projectID string
	table     string
}

var config = beaconConfig{
	projectID: os.Getenv("GOOGLE_CLOUD_PROJECT"),
	table:     os.Getenv("TABLE"),
}

func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if config.projectID == "" {
		http.Error(w, "Mandatory variable GOOGLE_CLOUD_PROJECT not set", http.StatusInternalServerError)
		return
	}
	if config.table == "" {
		http.Error(w, "Mandatory variable TABLE not set", http.StatusInternalServerError)
		return
	}
	refName, allele, coord, err := parseInput(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed parsing parameters: %v", err), http.StatusBadRequest)
		return
	}

	ctx := appengine.NewContext(r)
	exists, err := exists(ctx, refName, allele, coord)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed searching for genome: %v", err), http.StatusInternalServerError)
		return
	}

	type beaconResponse struct {
		XMLName struct{} `xml:"BEACONResponse"`
		Exists  bool     `xml:"exists"`
	}
	var resp beaconResponse
	resp.Exists = exists

	w.Header().Set("Content-Type", "application/xml")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("Failed writing response: %v", err), http.StatusInternalServerError)
		return
	}
}

func exists(ctx context.Context, refName string, allele string, coord int64) (bool, error) {
	bqclient, err := bigquery.NewClient(ctx, config.projectID)
	if err != nil {
		return false, fmt.Errorf("Failed to access data: %v", err)
	}

	// Start is inclusive, End is exclusive.  Search exactly for coordinate.
	query := fmt.Sprintf(`
		SELECT count(v.reference_name) as count
		FROM %s as v
		WHERE reference_name='%s'
			AND v.start <= %d AND %d < v.end
	 	 	AND reference_bases='%s'
		LIMIT 1`,
		fmt.Sprintf("`%s`", config.table),
		refName,
		coord,
		coord+1,
		allele)
	q := bqclient.Query(query)

	it, err := q.Read(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed querying data: %v", err)
	}

	type Result struct {
		Count int
	}
	var result Result
	if err := it.Next(&result); err != nil {
		return false, fmt.Errorf("Failed reading result: %v", err)
	}
	return result.Count > 0, nil
}

func parseInput(r *http.Request) (string, string, int64, error) {
	refName := r.FormValue("chromosome")
	if refName == "" {
		return "", "", 0, errors.New("chromosome name is required")
	}
	allele := r.FormValue("allele")
	if refName == "" {
		return "", "", 0, errors.New("allele is required")
	}
	coord, err := strconv.ParseInt(r.FormValue("coordinate"), 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("parsing coordinate: %v", err)
	}
	return refName, allele, coord, nil
}
