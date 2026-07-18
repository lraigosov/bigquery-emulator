package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	bigqueryv2 "google.golang.org/api/bigquery/v2"
)

func TestAppendValueToARROWBuilderList(t *testing.T) {
	mem := memory.NewGoAllocator()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "items", Type: arrow.ListOf(arrow.PrimitiveTypes.Int64), Nullable: true},
	}, nil)
	rb := array.NewRecordBuilder(mem, schema)
	defer rb.Release()

	listBuilder := rb.Field(0).(*array.ListBuilder)
	rows := []struct {
		cells   []*TableCell
		wantLen int
	}{
		{cells: []*TableCell{{V: "1"}, {V: "2"}, {V: "3"}}, wantLen: 3},
		{cells: []*TableCell{}, wantLen: 0},
		{cells: []*TableCell{{V: "4"}}, wantLen: 1},
		{cells: nil, wantLen: 0},
	}

	for _, row := range rows {
		if err := (&TableCell{V: row.cells}).AppendValueToARROWBuilder(listBuilder); err != nil {
			t.Fatalf("AppendValueToARROWBuilder: %v", err)
		}
	}

	record := rb.NewRecord()
	defer record.Release()
	if got := record.NumRows(); got != int64(len(rows)) {
		t.Fatalf("NumRows = %d, want %d", got, len(rows))
	}

	column := record.Column(0).(*array.List)
	for i, row := range rows {
		start, end := column.ValueOffsets(i)
		if got := int(end - start); got != row.wantLen {
			t.Errorf("row %d: list length = %d, want %d", i, got, row.wantLen)
		}
	}

	values := column.ListValues().(*array.Int64)
	start, _ := column.ValueOffsets(0)
	for i, want := range []int64{1, 2, 3} {
		if got := values.Value(int(start) + i); got != want {
			t.Errorf("row 0 element %d = %d, want %d", i, got, want)
		}
	}
}

func TestFormatCellHandlesNilField(t *testing.T) {
	cell := &TableCell{V: "value"}
	got := formatCell(nil, cell, true)
	if got != cell {
		t.Fatalf("formatCell(nil, cell) = %#v, want same cell pointer", got)
	}
}

func TestFormatCellRecordPointerFormatsNestedTimestamp(t *testing.T) {
	field := &bigqueryv2.TableFieldSchema{
		Type: "RECORD",
		Fields: []*bigqueryv2.TableFieldSchema{
			{Type: "TIMESTAMP"},
		},
	}
	row := &TableRow{
		F: []*TableCell{{V: "2026-06-15T00:00:00Z"}},
	}
	cell := &TableCell{V: row}

	got := formatCell(field, cell, true)
	gotRow, ok := got.V.(*TableRow)
	if !ok {
		t.Fatalf("got.V type = %T, want *TableRow", got.V)
	}
	if len(gotRow.F) != 1 {
		t.Fatalf("len(gotRow.F) = %d, want 1", len(gotRow.F))
	}
	tm, err := time.Parse(time.RFC3339, "2026-06-15T00:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse failed: %v", err)
	}
	want := fmt.Sprint(tm.UnixMicro())
	if gotRow.F[0].V != want {
		t.Fatalf("nested timestamp = %v, want %v", gotRow.F[0].V, want)
	}
}
