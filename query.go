/*
 * Copyright (C) 2018 Google Inc.
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
	tableId   string
}

const (
	projectKey = "GOOGLE_CLOUD_PROJECT"
	bqTableKey = "GOOGLE_BIGQUERY_TABLE"
)

var config = beaconConfig{
	projectID: os.Getenv(projectKey),
	tableId:   os.Getenv(bqTableKey),
}

func init() {
	http.HandleFunc("/query", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if err := validateServerConfig(); err != nil {
		http.Error(w, fmt.Sprintf("validating server configuration: %v", err), http.StatusInternalServerError)
		return
	}

	refName, allele, coord, err := parseInput(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("validating input: %v", err), http.StatusBadRequest)
		return
	}

	ctx := appengine.NewContext(r)
	exists, err := genomeExists(ctx, refName, allele, coord)
	if err != nil {
		http.Error(w, fmt.Sprintf("computing result: %v", err), http.StatusInternalServerError)
		return
	}

	if err := writeResponse(w, exists); err != nil {
		http.Error(w, fmt.Sprintf("validating server configuration: %v", err), http.StatusInternalServerError)
		return
	}
}

func genomeExists(ctx context.Context, refName string, allele string, coord int64) (bool, error) {
	// Start is inclusive, End is exclusive.  Search exactly for coordinate.
	query := fmt.Sprintf(`
		SELECT count(v.reference_name) as count
		FROM %s as v
		WHERE reference_name='%s'
			AND v.start <= %d AND %d < v.end
	 	 	AND reference_bases='%s'
		LIMIT 1`,
		fmt.Sprintf("`%s`", config.tableId),
		refName,
		coord,
		coord+1,
		allele)

	bqclient, err := bigquery.NewClient(ctx, config.projectID)
	if err != nil {
		return false, fmt.Errorf("creating bigquery client: %v", err)
	}
	it, err := bqclient.Query(query).Read(ctx)
	if err != nil {
		return false, fmt.Errorf("querying database: %v", err)
	}

	var result struct {
		Count int
	}
	if err := it.Next(&result); err != nil {
		return false, fmt.Errorf("reading query result: %v", err)
	}
	return result.Count > 0, nil
}

func validateServerConfig() error {
	if config.projectID == "" {
		return fmt.Errorf("%s must be specified", projectKey)
	}
	if config.tableId == "" {
		return fmt.Errorf("%s must be specified", bqTableKey)
	}
	return nil
}

func parseInput(r *http.Request) (string, string, int64, error) {
	refName := r.FormValue("chromosome")
	if refName == "" {
		return "", "", 0, errors.New("missing chromosome name")
	}
	allele := r.FormValue("allele")
	if refName == "" {
		return "", "", 0, errors.New("missing allele")
	}
	coord, err := strconv.ParseInt(r.FormValue("coordinate"), 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("parsing coordinate: %v", err)
	}
	return refName, allele, coord, nil
}

func writeResponse(w http.ResponseWriter, exists bool) error {
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
		return fmt.Errorf("serializing response: %v", err)
	}
	return nil
}
