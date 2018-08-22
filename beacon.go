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
	"html/template"
	"net/http"
	"os"
	"strconv"

	"cloud.google.com/go/bigquery"
	"google.golang.org/appengine"
)

type beaconConfig struct {
	ApiVersion string
	ProjectID  string
	TableId    string
}

const (
	apiVersionKey = "VERSION"
	projectKey    = "GOOGLE_CLOUD_PROJECT"
	bqTableKey    = "GOOGLE_BIGQUERY_TABLE"
)

var (
	aboutTemplate = template.Must(template.ParseFiles("about.xml"))
	config        = beaconConfig{
		ApiVersion: os.Getenv(apiVersionKey),
		ProjectID:  os.Getenv(projectKey),
		TableId:    os.Getenv(bqTableKey),
	}
)

func init() {
	http.HandleFunc("/", aboutBeacon)
	http.HandleFunc("/query", query)
}

func aboutBeacon(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	aboutTemplate.Execute(w, config)
}

func query(w http.ResponseWriter, r *http.Request) {
	if err := validateServerConfig(); err != nil {
		http.Error(w, fmt.Sprintf("validating server configuration: %v", err), http.StatusInternalServerError)
		return
	}

	params, err := parseInput(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("validating input: %v", err), http.StatusBadRequest)
		return
	}

	ctx := appengine.NewContext(r)
	exists, err := genomeExists(ctx, params)
	if err != nil {
		http.Error(w, fmt.Sprintf("computing result: %v", err), http.StatusInternalServerError)
		return
	}

	if err := writeResponse(w, exists); err != nil {
		http.Error(w, fmt.Sprintf("validating server configuration: %v", err), http.StatusInternalServerError)
		return
	}
}

func genomeExists(ctx context.Context, params queryParams) (bool, error) {
	var w where
	w.append(fmt.Sprintf("reference_name='%s'", params.refName))
	// Start is inclusive, End is exclusive.  Search exactly for coordinate.
	w.append(fmt.Sprintf("v.start <= %d AND %d < v.end", params.coord, params.coord+1))
	w.append(fmt.Sprintf("reference_bases='%s'", params.allele))

	query := fmt.Sprintf(`
		SELECT count(v.reference_name) as count
		FROM %s as v
		WHERE %s
		LIMIT 1`,
		fmt.Sprintf("`%s`", config.TableId),
		w.clause)

	bqclient, err := bigquery.NewClient(ctx, config.ProjectID)
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

type where struct {
	clause string
}

func (w *where) append(statement string) {
	if statement == "" {
		return
	}
	var conj string
	if len(w.clause) > 0 {
		conj = " AND "
	}
	w.clause = fmt.Sprintf("%s%s(%s)", w.clause, conj, statement)
}

func validateServerConfig() error {
	if config.ProjectID == "" {
		return fmt.Errorf("%s must be specified", projectKey)
	}
	if config.TableId == "" {
		return fmt.Errorf("%s must be specified", bqTableKey)
	}
	return nil
}

type queryParams struct {
	refName string
	allele  string
	coord   int64
}

func parseInput(r *http.Request) (queryParams, error) {
	refName := r.FormValue("chromosome")
	if refName == "" {
		return queryParams{}, errors.New("missing chromosome name")
	}
	allele := r.FormValue("allele")
	if refName == "" {
		return queryParams{}, errors.New("missing allele")
	}
	coord, err := strconv.ParseInt(r.FormValue("coordinate"), 10, 64)
	if err != nil {
		return queryParams{}, fmt.Errorf("parsing coordinate: %v", err)
	}
	return queryParams{refName, allele, coord}, nil
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
