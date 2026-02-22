package grafana

import (
	"fmt"
)

// FrameRows extracts tabular rows from Grafana /api/ds/query response payload.
func FrameRows(resp map[string]any) ([]map[string]any, error) {
	results, ok := resp["results"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response has no results")
	}
	rows := make([]map[string]any, 0)
	for _, v := range results {
		resObj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		frames, ok := resObj["frames"].([]any)
		if !ok {
			continue
		}
		for _, f := range frames {
			frame, ok := f.(map[string]any)
			if !ok {
				continue
			}
			fr, err := frameToRows(frame)
			if err != nil {
				continue
			}
			rows = append(rows, fr...)
		}
	}
	return rows, nil
}

func frameToRows(frame map[string]any) ([]map[string]any, error) {
	schema, _ := frame["schema"].(map[string]any)
	fieldsAny, _ := schema["fields"].([]any)
	if len(fieldsAny) == 0 {
		return nil, fmt.Errorf("no fields")
	}
	fieldNames := make([]string, 0, len(fieldsAny))
	for _, f := range fieldsAny {
		fm, _ := f.(map[string]any)
		name, _ := fm["name"].(string)
		if name == "" {
			name = "value"
		}
		fieldNames = append(fieldNames, name)
	}

	data, _ := frame["data"].(map[string]any)
	valuesAny, _ := data["values"].([]any)
	if len(valuesAny) == 0 {
		return nil, fmt.Errorf("no values")
	}

	columns := make([][]any, 0, len(valuesAny))
	maxRows := 0
	for _, colAny := range valuesAny {
		col, _ := colAny.([]any)
		columns = append(columns, col)
		if len(col) > maxRows {
			maxRows = len(col)
		}
	}
	rows := make([]map[string]any, 0, maxRows)
	for i := 0; i < maxRows; i++ {
		row := map[string]any{}
		for cIdx, col := range columns {
			if cIdx >= len(fieldNames) {
				continue
			}
			if i < len(col) {
				row[fieldNames[cIdx]] = col[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
