package store

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed catalog/practice.json
var practiceData []byte

func (s *Store) migrateCatalog() error {
	for table, fields := range map[string]map[string]string{
		"labs":      {"archived": "INTEGER NOT NULL DEFAULT 0", "catalog_key": "TEXT NOT NULL DEFAULT ''", "document_url": "TEXT NOT NULL DEFAULT ''"},
		"exercises": {"kind": "TEXT NOT NULL DEFAULT 'python'", "catalog_key": "TEXT NOT NULL DEFAULT ''", "archived": "INTEGER NOT NULL DEFAULT 0"},
	} {
		rows, err := s.DB.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			return err
		}
		present := map[string]bool{}
		for rows.Next() {
			var cid, nn, pk int
			var name, typ string
			var def any
			if err = rows.Scan(&cid, &name, &typ, &nn, &def, &pk); err != nil {
				rows.Close()
				return err
			}
			present[name] = true
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for name, declaration := range fields {
			if !present[name] {
				if _, err = s.DB.Exec("ALTER TABLE " + table + " ADD COLUMN " + name + " " + declaration); err != nil {
					return err
				}
			}
		}
	}
	_, err := s.DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS labs_catalog_key ON labs(catalog_key) WHERE catalog_key<>''; CREATE UNIQUE INDEX IF NOT EXISTS exercises_catalog_key ON exercises(catalog_key) WHERE catalog_key<>'';`)
	return err
}

// Seed synchronizes the supplied curriculum once per version. Existing IDs,
// submissions and drafts survive; old labs are archived rather than deleted.
func (s *Store) Seed() error {
	var data struct {
		Version string `json:"version"`
		Labs    []struct {
			Lab
			Key       string     `json:"key"`
			Exercises []Exercise `json:"exercises"`
		} `json:"labs"`
	}
	if err := json.Unmarshal(practiceData, &data); err != nil {
		return err
	}
	if len(data.Labs) != 4 || data.Version == "" {
		return fmt.Errorf("invalid practice catalog")
	}
	var current string
	s.DB.QueryRow(`SELECT value FROM settings WHERE key='practice_catalog_version'`).Scan(&current)
	if current == data.Version {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE labs SET archived=1`); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE exercises SET archived=1"); err != nil {
		return err
	}
	for _, lab := range data.Labs {
		var lid int64
		err = tx.QueryRow(`SELECT id FROM labs WHERE catalog_key=?`, lab.Key).Scan(&lid)
		if err == sql.ErrNoRows {
			res, e := tx.Exec(`INSERT INTO labs(title,description,position,source,archived,catalog_key,document_url) VALUES(?,?,?,'practice',0,?,?)`, lab.Title, lab.Description, lab.Position, lab.Key, lab.DocumentURL)
			if e != nil {
				return e
			}
			lid, _ = res.LastInsertId()
		} else if err != nil {
			return err
		} else {
			if _, err = tx.Exec(`UPDATE labs SET title=?,description=?,position=?,source='practice',archived=0,document_url=? WHERE id=?`, lab.Title, lab.Description, lab.Position, lab.DocumentURL, lid); err != nil {
				return err
			}
		}
		for _, exercise := range lab.Exercises {
			var eid int64
			err = tx.QueryRow(`SELECT id FROM exercises WHERE catalog_key=?`, exercise.CatalogKey).Scan(&eid)
			if err == sql.ErrNoRows {
				_, err = tx.Exec(`INSERT INTO exercises(lab_id,title,statement,template,tests,timeout_ms,memory_mb,difficulty,position,kind,catalog_key) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, lid, exercise.Title, exercise.Statement, exercise.Template, exercise.Tests, exercise.TimeoutMS, exercise.MemoryMB, exercise.Difficulty, exercise.Position, exercise.Kind, exercise.CatalogKey)
			} else if err == nil {
				_, err = tx.Exec(`UPDATE exercises SET lab_id=?,title=?,statement=?,template=?,tests=?,timeout_ms=?,memory_mb=?,difficulty=?,position=?,kind=?,archived=0 WHERE id=?`, lid, exercise.Title, exercise.Statement, exercise.Template, exercise.Tests, exercise.TimeoutMS, exercise.MemoryMB, exercise.Difficulty, exercise.Position, exercise.Kind, eid)
			}
			if err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(`INSERT INTO settings(key,value) VALUES('practice_catalog_version',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, data.Version); err != nil {
		return err
	}
	return tx.Commit()
}
