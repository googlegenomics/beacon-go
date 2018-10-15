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

// Package variants contains support for variant specific operations.
package variants

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

// Query holds information about a single query against a Beacon.
type Query struct {
	// RefName is the chromosome reference name.
	RefName string
	// Allele is the allele reference base.
	Allele string
	// Coord is the coordinate that intersects the retrieved alleles.
	Coord *int64
}

// Execute queries the allele database with the Query parameters.
func (q *Query) Execute(ctx context.Context, client *bigquery.Client, tableID string) (bool, error) {
	query := fmt.Sprintf(`
		SELECT count(v.reference_name) as count
		FROM %s as v
		WHERE %s
		LIMIT 1`,
		fmt.Sprintf("`%s`", tableID),
		q.whereClause(),
	)

	it, err := client.Query(query).Read(ctx)
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

// ValidateInput validates the Query parameters meet the ga4gh beacon api requirements.
func (q *Query) ValidateInput() error {
	if q.RefName == "" {
		return errors.New("missing chromosome name")
	}
	if q.Allele == "" {
		return errors.New("missing allele")
	}
	if q.Coord == nil {
		return errors.New("missing coordinate")
	}
	return nil
}

func (q *Query) whereClause() string {
	var clauses []string
	add := func(format string, args ...interface{}) {
		clauses = append(clauses, fmt.Sprintf(format, args...))
	}
	simpleClause := func(dbColumn, value string) {
		if dbColumn != "" && value != "" {
			add("%s='%s'", dbColumn, value)
		}
	}
	simpleClause("reference_name", q.RefName)
	simpleClause("reference_bases", q.Allele)
	// Start is inclusive, End is exclusive.  Search exactly for coordinate.
	if q.Coord != nil {
		add("v.start <= %d AND %d < v.end", *q.Coord, *q.Coord+1)
	}
	return strings.Join(clauses, " AND ")
}
