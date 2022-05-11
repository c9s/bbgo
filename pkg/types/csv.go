package types

// CsvFormatter is an interface used for dumping object into csv file
type CsvFormatter interface {
	CsvHeader() []string
	CsvRecords() [][]string
}
