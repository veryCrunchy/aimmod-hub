package store

import (
	"context"
	"fmt"
	"strings"
)

type ProfileMeta struct {
	Handle        string
	DisplayName   string
	RunCount      uint32
	ScenarioCount uint32
}

type ScenarioMeta struct {
	Name      string
	RunCount  uint32
	BestScore float64
	AvgAcc    float64
}

type RunMeta struct {
	ScenarioName    string
	UserHandle      string
	UserDisplayName string
	Score           float64
	Accuracy        float64
}

func (s *Store) GetProfileMeta(ctx context.Context, handle string) (*ProfileMeta, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return nil, fmt.Errorf("handle required")
	}
	var m ProfileMeta
	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)),
			COUNT(sr.session_id),
			COUNT(DISTINCT sr.scenario_name)
		FROM hub_users hu
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		LEFT JOIN scenario_runs sr ON sr.user_id = hu.id
		WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
		   OR LOWER(hu.external_id) = $1
		GROUP BY la.username, hu.external_id, la.display_name
		LIMIT 1
	`, handle).Scan(&m.Handle, &m.DisplayName, &m.RunCount, &m.ScenarioCount)
	if err != nil {
		return nil, fmt.Errorf("get profile meta: %w", err)
	}
	return &m, nil
}

func (s *Store) GetScenarioMeta(ctx context.Context, slug string) (*ScenarioMeta, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT scenario_name FROM scenario_runs ORDER BY scenario_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}
	defer rows.Close()

	scenarioName := ""
	for rows.Next() {
		var candidate string
		if err := rows.Scan(&candidate); err != nil {
			return nil, err
		}
		if slugifyScenarioName(candidate) == slug {
			scenarioName = candidate
			break
		}
	}
	rows.Close()

	if scenarioName == "" {
		return nil, fmt.Errorf("scenario not found")
	}

	var m ScenarioMeta
	m.Name = scenarioName
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(MAX(score), 0), COALESCE(AVG(accuracy), 0)
		FROM scenario_runs
		WHERE scenario_name = $1
	`, scenarioName).Scan(&m.RunCount, &m.BestScore, &m.AvgAcc); err != nil {
		return nil, fmt.Errorf("get scenario meta: %w", err)
	}
	return &m, nil
}

func (s *Store) GetRunMeta(ctx context.Context, runID string) (*RunMeta, error) {
	var m RunMeta
	err := s.pool.QueryRow(ctx, `
		SELECT
			sr.scenario_name,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)),
			sr.score,
			sr.accuracy
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE sr.public_run_id = $1 OR sr.session_id = $1
		LIMIT 1
	`, runID).Scan(&m.ScenarioName, &m.UserHandle, &m.UserDisplayName, &m.Score, &m.Accuracy)
	if err != nil {
		return nil, fmt.Errorf("get run meta: %w", err)
	}
	return &m, nil
}
