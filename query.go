package beacon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

type Query struct {
	refName string
	allele  string
	coord   *int64
}

func (q *Query) Execute(ctx context.Context, projectID, tableID string) (bool, error) {
	query := fmt.Sprintf(`
		SELECT count(v.reference_name) as count
		FROM %s as v
		WHERE %s
		LIMIT 1`,
		fmt.Sprintf("`%s`", tableID),
		q.whereClause())
	bqclient, err := bigquery.NewClient(ctx, projectID)
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

func (q *Query) ValidateInput() error {
	if q.refName == "" {
		return errors.New("missing chromosome name")
	}
	if q.allele == "" {
		return errors.New("missing allele")
	}
	if q.coord == nil {
		return errors.New("missing coordinate")
	}
	return nil
}

func (q *Query) whereClause() string {
	var clauses []string
	add := func(clause string) {
		if clause != "" {
			clauses = append(clauses, clause)
		}
	}
	simpleClause := func(dbColumn, value string) {
		if dbColumn != "" && value != "" {
			add(fmt.Sprintf("%s='%s'", dbColumn, value))
		}
	}
	simpleClause("reference_name", q.refName)
	simpleClause("reference_bases", q.allele)
	// Start is inclusive, End is exclusive.  Search exactly for coordinate.
	if q.coord != nil {
		add(fmt.Sprintf("v.start <= %d AND %d < v.end", *q.coord, *q.coord+1))
	}
	return strings.Join(clauses, " AND ")
}
