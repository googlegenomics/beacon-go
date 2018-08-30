package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

// Query holds information about a single query against a Beacon.
type Query struct {
	// RefName - the chromosome reference name.
	RefName string
	// Allele - the allele reference base.
	Allele string
	// Coord - the coordinate that intersects the retrieved alleles.
	Coord *int64
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
