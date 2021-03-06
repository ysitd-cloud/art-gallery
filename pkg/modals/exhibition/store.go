package exhibition

import (
	"context"
	"database/sql"

	"golang.ysitd.cloud/db"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

const hostPathQuery = `
WITH recursive union_revision AS (
  WITH target_exhibition AS (
    SELECT exhibition.id, revision, cors FROM exhibition
      INNER JOIN exhibition_host ON exhibition_host.exhibition = exhibition.id
    WHERE hostname = $1
  ),
    linked_revision AS (
      SELECT r.id, parent, commit_time, e.cors, e.id AS exhibition FROM revision r
      INNER JOIN target_exhibition e ON r.id = e.revision
    )
  SELECT id, parent, commit_time, exhibition, cors FROM linked_revision
  UNION ALL
  SELECT r.id, r.parent, r.commit_time, r.exhibition, r.cors FROM linked_revision r
    INNER JOIN linked_revision p on p.parent = r.id
)
SELECT r.commit_time, tree.pathname, tree.hash, r.exhibition, r.cors FROM union_revision r
  INNER JOIN tree on tree.revision = r.id
WHERE tree.pathname = $2
ORDER BY r.commit_time DESC
LIMIT 1
`

const hostQuery = "SELECT exhibition, cors FROM exhibition_host WHERE hostname = $1"

type Store struct {
	Opener *db.GeneralOpener  `inject:""`
	Cache  *cache.Cache       `inject:""`
	Logger logrus.FieldLogger `inject:"exhibition logger"`
}

func (s *Store) GetExhibitionWithHost(ctx context.Context, hostname string) (e *Exhibition, err error) {
	cacheKey := hostname + "/*"
	if val, hit := s.Cache.Get(cacheKey); hit {
		s.Logger.Debugf("Load %s from cache", hostname)
		return val.(*Exhibition), nil
	}
	s.Logger.Debugf("Load %s from database", hostname)
	e, err = s.getFromDB(ctx, hostname)
	if err != nil {
		return
	}

	return
}

func (s *Store) GetExhibitionWithPath(ctx context.Context, hostname, path string) (e *Exhibition, err error) {
	cacheKey := hostname + "/" + path
	if val, hit := s.Cache.Get(cacheKey); hit {
		s.Logger.Debugf("Load %s%s from cache", hostname, path)
		return val.(*Exhibition), nil
	}

	s.Logger.Debugf("Load %s%s from database", hostname, path)
	e, err = s.getFromDBWithPath(ctx, hostname, path)
	if err != nil {
		return
	}
	s.Cache.SetDefault(cacheKey, e)
	return
}

func (s *Store) getFromDB(ctx context.Context, hostname string) (e *Exhibition, err error) {
	conn, err := s.Opener.Open()
	if err != nil {
		return
	}
	defer conn.Close()

	row := conn.QueryRowContext(ctx, hostQuery, hostname)

	var instance Exhibition

	if err := row.Scan(&instance.CommitTime, &instance.Pathname, &instance.Hash, &instance.ID, &instance.CORS); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	e = &instance

	return
}

func (s *Store) getFromDBWithPath(ctx context.Context, hostname, path string) (e *Exhibition, err error) {
	conn, err := s.Opener.Open()
	if err != nil {
		return
	}
	defer conn.Close()

	row := conn.QueryRowContext(ctx, hostPathQuery, hostname, path)

	var instance Exhibition

	if err := row.Scan(&instance.CommitTime, &instance.Pathname, &instance.Hash, &instance.ID, &instance.CORS); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	e = &instance

	return
}
