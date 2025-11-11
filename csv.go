package main

import (
	"encoding/csv"
	"io"
	"reflect"
	"strings"
)

// Unmarshal parses the CSV data into a slice of structs.
func Unmarshal(reader io.Reader, v interface{}) error {
	r := csv.NewReader(reader)
	records, err := r.ReadAll()
	if err != nil {
		return err
	}
	slice := reflect.ValueOf(v).Elem()
	slice.Set(reflect.MakeSlice(slice.Type(), len(records)-1, len(records)-1))
	itemType := slice.Type().Elem()
	for i, record := range records {
		if i == 0 {
			continue
		}
		item := reflect.New(itemType.Elem())
		for j, value := range record {
			header := strings.TrimSpace(records[0][j])
			field := item.Elem().FieldByName(header)
			if !field.IsValid() {
				// try to find the field by json tag
				for i := 0; i < item.Elem().NumField(); i++ {
					f := item.Elem().Type().Field(i)
					tag := f.Tag.Get("json")
					if tag == header {
						field = item.Elem().Field(i)
						break
					}
				}
			}
			if field.IsValid() {
				field.SetString(value)
			}
		}
		slice.Index(i - 1).Set(item)
	}
	return nil
}
