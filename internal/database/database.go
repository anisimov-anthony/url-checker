package database

import (
	"database/sql"
	"fmt"
	"time"

	"url-checker/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{db: db}

	if err := database.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return database, nil
}

func (d *Database) createTables() error {
	batchSQL := `CREATE TABLE IF NOT EXISTS batches (
		links_num INTEGER PRIMARY KEY,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);`

	if _, err := d.db.Exec(batchSQL); err != nil {
		return fmt.Errorf("failed to create batches table: %w", err)
	}

	linkSQL := `CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		status TEXT NOT NULL,
		batch_num INTEGER NOT NULL,
		time DATETIME,
		FOREIGN KEY (batch_num) REFERENCES batches(links_num)
	);`

	if _, err := d.db.Exec(linkSQL); err != nil {
		return fmt.Errorf("failed to create links table: %w", err)
	}

	return nil
}

func (d *Database) CreateBatch(linksNum int, status models.BatchStatus, createdAt time.Time) error {
	sql := `INSERT INTO batches (links_num, status, created_at) VALUES (?, ?, ?)`

	_, err := d.db.Exec(sql, linksNum, status, createdAt)
	if err != nil {
		return fmt.Errorf("failed to create batch: %w", err)
	}

	return nil
}

func (d *Database) CreateLink(url string, status models.LinkStatus, batchNum int, time *time.Time) (int, error) {
	sql := `INSERT INTO links (url, status, batch_num, time) VALUES (?, ?, ?, ?)`

	result, err := d.db.Exec(sql, url, status, batchNum, time)
	if err != nil {
		return 0, fmt.Errorf("failed to create link: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get link id: %w", err)
	}

	return int(id), nil
}

func (d *Database) UpdateLinkStatus(id int, status models.LinkStatus, time *time.Time) error {
	sql := `UPDATE links SET status = ?, time = ? WHERE id = ?`

	_, err := d.db.Exec(sql, status, time, id)
	if err != nil {
		return fmt.Errorf("failed to update link status: %w", err)
	}

	return nil
}

func (d *Database) UpdateBatchStatus(linksNum int, status models.BatchStatus) error {
	sql := `UPDATE batches SET status = ? WHERE links_num = ?`

	_, err := d.db.Exec(sql, status, linksNum)
	if err != nil {
		return fmt.Errorf("failed to update batch status: %w", err)
	}

	return nil
}

func (d *Database) GetLinksByBatchNum(linksNum int) ([]*models.Link, error) {
	sql := `SELECT id, url, status, batch_num, time FROM links WHERE batch_num = ? ORDER BY id`

	rows, err := d.db.Query(sql, linksNum)
	if err != nil {
		return nil, fmt.Errorf("failed to query links: %w", err)
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link := &models.Link{}
		err := rows.Scan(&link.ID, &link.URL, &link.Status, &link.BatchNum, &link.Time)
		if err != nil {
			return nil, fmt.Errorf("failed to scan link: %w", err)
		}
		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return links, nil
}

func (d *Database) GetBatch(linksNum int) (*models.Batch, error) {
	sql := `SELECT links_num, status, created_at FROM batches WHERE links_num = ?`

	batch := &models.Batch{}
	err := d.db.QueryRow(sql, linksNum).Scan(&batch.LinksNum, &batch.Status, &batch.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, fmt.Errorf("batch not found")
		}
		return nil, fmt.Errorf("failed to query batch: %w", err)
	}

	return batch, nil
}

func (d *Database) GetAllBatches() ([]*models.Batch, error) {
	sql := `SELECT links_num, status, created_at FROM batches ORDER BY links_num`

	rows, err := d.db.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to query batches: %w", err)
	}
	defer rows.Close()

	var batches []*models.Batch
	for rows.Next() {
		batch := &models.Batch{}
		err := rows.Scan(&batch.LinksNum, &batch.Status, &batch.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan batch: %w", err)
		}
		batches = append(batches, batch)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return batches, nil
}

func (d *Database) GetMaxBatchNum() (int, error) {
	sql := `SELECT COALESCE(MAX(links_num), 0) FROM batches`

	var maxID int
	err := d.db.QueryRow(sql).Scan(&maxID)
	if err != nil {
		return 0, fmt.Errorf("failed to get max batch num: %w", err)
	}

	return maxID, nil
}

func (d *Database) GetBatchesByIDs(batchIDs []int) ([]*models.Batch, []*models.Link, error) {
	if len(batchIDs) == 0 {
		return nil, nil, fmt.Errorf("no batch IDs provided")
	}

	batchSQL := `SELECT links_num, status, created_at FROM batches WHERE links_num IN (`
	args := make([]any, len(batchIDs))
	for i, id := range batchIDs {
		if i > 0 {
			batchSQL += ","
		}
		batchSQL += "?"
		args[i] = id
	}
	batchSQL += ") ORDER BY links_num"

	batchRows, err := d.db.Query(batchSQL, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query batches: %w", err)
	}
	defer batchRows.Close()

	var batches []*models.Batch
	for batchRows.Next() {
		batch := &models.Batch{}
		err := batchRows.Scan(&batch.LinksNum, &batch.Status, &batch.CreatedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan batch: %w", err)
		}
		batches = append(batches, batch)
	}

	if err := batchRows.Err(); err != nil {
		return nil, nil, err
	}

	linkSQL := `SELECT id, url, status, batch_num, time FROM links WHERE batch_num IN (`
	linkArgs := make([]any, len(batchIDs))
	for i, id := range batchIDs {
		if i > 0 {
			linkSQL += ","
		}
		linkSQL += "?"
		linkArgs[i] = id
	}
	linkSQL += ") ORDER BY batch_num, id"

	linkRows, err := d.db.Query(linkSQL, linkArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query links: %w", err)
	}
	defer linkRows.Close()

	var links []*models.Link
	for linkRows.Next() {
		link := &models.Link{}
		err := linkRows.Scan(&link.ID, &link.URL, &link.Status, &link.BatchNum, &link.Time)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan link: %w", err)
		}
		links = append(links, link)
	}

	if err := linkRows.Err(); err != nil {
		return nil, nil, err
	}

	return batches, links, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}
