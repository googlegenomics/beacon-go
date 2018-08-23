package beacon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

type Query struct {
	RefName  string
	RefBases string
	Start    *int64
	End      *int64
}

func (q *Query) Execute(ctx context.Context) (bool, error) {
	query := fmt.Sprintf(`
		SELECT count(v.reference_name) as count
		FROM %s as v
		WHERE %s
		LIMIT 1`,
		fmt.Sprintf("`%s`", config.TableID),
		q.whereClause())

	bqClient, err := bigquery.NewClient(ctx, config.ProjectID)
	if err != nil {
		return false, fmt.Errorf("creating bigquery client: %v", err)
	}
	it, err := bqClient.Query(query).Read(ctx)
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

func (q *Query) validateInput() error {
	if q.RefName == "" {
		return errors.New("missing referenceName")
	}
	if q.RefBases == "" {
		return errors.New("missing referenceBases")
	}

	if err := q.validateCoordinates(); err != nil {
		return fmt.Errorf("validating coordinates: %v", err)
	}
	return nil
}

func (q *Query) validateCoordinates() error {
	if q.Start != nil && (q.End != nil || q.RefBases != "") {
		return nil
	}
	return errors.New("coordinate requirements not met")
}

func (q *Query) whereClause() string {
	var clauses []string
	add := func(clause string) {
		if clause != "" {
			clauses = append(clauses, clause)
		}
	}
	add(fmt.Sprintf("reference_name='%s'", q.RefName))
	add(fmt.Sprintf("reference_bases='%s'", q.RefBases))
	add(q.bqCoordinatesToWhereClause())
	return strings.Join(clauses, " AND ")
}

func (q *Query) bqCoordinatesToWhereClause() string {
	if q.Start != nil {
		if q.End != nil {
			return fmt.Sprintf("v.start = %d AND %d = v.end", *q.Start, *q.End)
		}
		return fmt.Sprintf("v.start = %d", *q.Start)
	}
	return ""
}
