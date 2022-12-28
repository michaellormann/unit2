package leprechaun

/* This file is part of Leprechaun.
*  @author: Michael Lormann
 */
import (
	"log"
	"os"
	"path/filepath"

	"database/sql"
	// go-sqlite3 is imported for its side-effect of loading the sqlite3 driver.
	_ "github.com/mattn/go-sqlite3"
)

// SQLITE operations.
var (
	sqlDatabaseName        = "Leprechaun.Ledger"
	databaseInit    string = "CREATE TABLE RECORDS (ASSET, COST, ID, PRICE, SALE_ID, SOLD, STATUS, TIMESTAMP, VOLUME, TYPE, TRIGGER_PRICE)"
	recordInsert           = "INSERT INTO RECORDS VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	idSearch        string = "SELECT * FROM RECORDS WHERE ID = ?"
	// abs(PRICE) + abs(PRICE) * `margin` adjusts the price by profit margin provided.
	// E.g. to adjust a price of 2_000_000 by a 1% margin, we have 2_000_000 + (2_000_000 * 0.01)
	// giving an adjusted price of 2_020_000
	viableRecordSearch = "SELECT * FROM RECORDS WHERE ASSET = ? AND abs(PRICE) + abs(PRICE) * ? < ?"
	getAllRecordsOp    = "SELECT * FROM RECORDS"
	typeSearchOp       = "SELECT * FROM RECORDS WHERE ASSET = ? AND TYPE = ?"
	deleteRecordOp     = "DELETE FROM RECORDS WHERE ID = ?"
)

// Ledger2 object stores records of purchased assets in a sql database.
type Ledger2 struct {
	databasePath string
	db           *sql.DB
	isOpen       bool
}

func GetLedger2() *Ledger2 {
	l := &Ledger2{databasePath: "."}
	l.loadDatabase()
	return l
}

// ViableRecords checks the database for any records whose prices are lower
// (beyond a certain `margin`) than the value of `price`.
func (l *Ledger2) ViableRecords(asset string, price float64) (records []Entry, err error) {
	// TODO:: Include margin test in viable records check
	if !l.isOpen {
		l.loadDatabase()
	}
	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	stmt, err := l.db.Prepare(viableRecordSearch)
	if err != nil {
		return
	}
	defer stmt.Close()
	margin := globalConfig.ProfitMargin
	rows, err := stmt.Query(asset, margin, price)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		rec := Entry{}
		err = scanEntryRows(rows, rec)
		if err != nil {
			return
		}
		records = append(records, rec)
	}
	tx.Commit()
	return
}

func scanEntryRows(rows *sql.Rows, rec Entry) (err error) {
	err = rows.Scan(&rec.Asset, &rec.PurchaseCost, &rec.SaleCost, &rec.ID, &rec.PurchasePrice, &rec.SalePrice, &rec.SaleID,
		&rec.Status, &rec.Timestamp, &rec.PurchaseVolume, &rec.SaleVolume, &rec.Profit, &rec.Type, &rec.TriggerPrice, &rec.Updated)
	return err
}

// GetRecordByID returns a record from the database with the `id` provided.
func (l *Ledger2) GetRecordByID(id string) (rec Entry, err error) {
	rec = Entry{}
	if !l.isOpen {
		l.loadDatabase()
	}
	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	stmt, err := l.db.Prepare(idSearch)
	if err != nil {
		return
	}
	defer stmt.Close()
	err = stmt.QueryRow(id).Scan(&rec.Asset, &rec.PurchaseCost, &rec.SaleCost, &rec.ID, &rec.PurchasePrice, &rec.SalePrice, &rec.SaleID,
		&rec.Status, &rec.Timestamp, &rec.PurchaseVolume, &rec.SaleVolume, &rec.Profit, &rec.Type, &rec.TriggerPrice, &rec.Updated)
	if err != nil {
		return
	}
	tx.Commit()

	return
}

// DeleteRecord removes the record with the provided `ID` from the ledger.
func (l *Ledger2) DeleteRecord(id string) (err error) {
	if !l.isOpen {
		l.loadDatabase()
	}
	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	stmt, err := l.db.Prepare(deleteRecordOp)
	if err != nil {
		return
	}
	defer stmt.Close()
	if err != nil {
		return
	}
	res, err := stmt.Exec(id)
	if err != nil {
		return
	}
	log.Printf("delete op: %v for record with id %s", res, id)
	tx.Commit()
	return
}

// GetRecordsByType retrieves records in the ledger by order type
func (l *Ledger2) GetRecordsByType(asset string, orderType Order) (records []Entry, err error) {
	if !l.isOpen {
		l.loadDatabase()
	}
	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	stmt, err := l.db.Prepare(typeSearchOp)
	if err != nil {
		return records, nil
	}
	defer stmt.Close()
	rows, err := stmt.Query(asset, orderType)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		rec := Entry{}
		err = scanEntryRows(rows, rec)
		if err != nil {
			return
		}
		records = append(records, rec)
	}
	tx.Commit()
	return
}

// AllRecords returns all purchase records stored in the ledger.
func (l *Ledger2) AllRecords() (records []Entry, err error) {
	if !l.isOpen {
		l.loadDatabase()
	}
	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	stmt, err := l.db.Prepare(getAllRecordsOp)
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		rec := Entry{}
		err = scanEntryRows(rows, rec)
		if err != nil {
			return
		}
		records = append(records, rec)
	}
	tx.Commit()
	return
}

// AddRecord adds a `Entry` to the database.
func (l *Ledger2) AddRecord(rec Entry) (err error) {
	if !l.isOpen {
		l.loadDatabase()
	}
	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	stmt, err := tx.Prepare(recordInsert)
	if err != nil {
		return
	}
	defer stmt.Close()
	_, err = stmt.Exec(&rec.Asset, &rec.PurchaseCost, &rec.SaleCost, &rec.ID, &rec.PurchasePrice, &rec.SalePrice, &rec.SaleID,
		&rec.Status, &rec.Timestamp, &rec.PurchaseVolume, &rec.SaleVolume, &rec.Profit, &rec.Type, &rec.TriggerPrice, &rec.Updated)
	if err != nil {
		log.Fatal(err)
		return err
	}
	tx.Commit()
	return
}

// Save closese the database. Must be called by any external user of the ledger.
func (l *Ledger2) Save() (err error) {
	if !l.isOpen {
		l.db.Close()
	}
	l.isOpen = false
	return
}

func (l *Ledger2) loadDatabase() {
	dataDir := filepath.Dir(l.databasePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		// log.Println("Data folder already exists.")
	}
	// first check if ledger db already exists
	alreadyExists := exists(l.databasePath)

	// open the database
	db, err := sql.Open("sqlite3", l.databasePath)
	if err != nil {
		log.Fatal(err)
	}
	if !alreadyExists {
		// We are just creating a new ledger
		_, err = db.Exec(databaseInit)
		if err != nil {
			log.Fatal("Could not initialize ledger database", err)
		}
	}
	l.db = db
	l.isOpen = true
	return
}

type OrderEntry struct {
	AssetName string
	OrderID   string
	Timestamp string
	Price     float64
	Volume    float64
}

type StopOrderEntry struct {
	OrderEntry
}

// AssetStats holds all time stats for an asset
type EntryStats struct {
	Asset                 string
	AllTimePurchaseVolume string
	AllTimeSalesVolume    string
	AllTimeSalesCost      string
	AllTimePurchasesCost  string
	AllTimeProfit         string
}

// RecordStack holds a FIFO stack of at most `maxRecordsToSave` `Entry` elements.
type EntryStack struct {
	records []Entry
}

var maxRecordsToSave int = 100

// appendRecord appends a value of type T to a FIFO stack (actually a slice),
//
//	with a max capacity of `maxRecordsToSave`
//
// If the stacked is filled, it's first value is popped. Note, the slice
// shouldn't be created with make(), but initialized like so: stack := []T
func (st *EntryStack) appendRecord(rec Entry) {
	lnt := len(st.records)
	if lnt >= maxRecordsToSave {
		x := lnt - maxRecordsToSave
		st.records = st.records[x+1 : lnt] // pop the first `x` elements
	}
	st.records = append(st.records, rec)
}
