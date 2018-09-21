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
	// Start matches the alleles that start at this position.
	Start *int64
	// End matches the alleles that end at this position.
	End *int64
	// StartMin matches the alleles that start at this position or higher.
	StartMin *int64
	// StartMax matches the alleles that start at this position or lower.
	StartMax *int64
	// EndMin matches the alleles that end at this position or higher.
	EndMin *int64
	// EndMax matches the alleles that end at this position or lower.
	EndMax *int64
}

// Execute queries the allele database with the Query parameters.
func (q *Query) Execute(ctx context.Context, projectID, tableID string) (bool, error) {
	query := fmt.Sprintf(`
		SELECT count(v.reference_name) as count
		FROM %s as v
		WHERE %s
		LIMIT 1`,
		fmt.Sprintf("`%s`", tableID),
		q.whereClause(),
	)

	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("creating bigquery client: %v", err)
	}
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
	if err := q.validateCoordinates(); err != nil {
		return fmt.Errorf("validating coordinates: %v", err)
	}
	return nil
}

func (q *Query) validateCoordinates() error {
	var precisePosition, imprecisePosition bool
	if q.Start != nil && (q.End != nil || q.Allele != "") {
		precisePosition = true
	}
	if q.StartMin != nil && q.StartMax != nil && q.EndMin != nil && q.EndMax != nil {
		imprecisePosition = true
	}

	if precisePosition && imprecisePosition {
		return errors.New("please query either precise or imprecise position")
	}
	if precisePosition || imprecisePosition {
		return nil
	}
	if q.Start != nil && q.End != nil || q.StartMin != nil || q.StartMax != nil || q.EndMin != nil || q.EndMax != nil {
		return errors.New("restrictions not met for provided coordinates")
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
	q.bqCoordinatesToWhereClause(add)
	return strings.Join(clauses, " AND ")
}

func (q *Query) bqCoordinatesToWhereClause(add func(format string, args ...interface{})) {
	if q.Start != nil {
		if q.End != nil {
			add("v.start = %d AND %d = v.end", *q.Start, *q.End)
		} else {
			add("v.start = %d", *q.Start)
		}
	}
	if q.StartMin != nil && q.StartMax != nil && q.EndMin != nil && q.EndMax != nil {
		add("%d <= v.start AND v.start <= %d AND %d <= v.end AND v.end <= %d", *q.StartMin, *q.StartMax, *q.EndMin, *q.EndMax)
	}
}
