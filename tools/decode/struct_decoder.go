package decode

import (
	"encoding/json"
	"fmt"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/mitchellh/mapstructure"
	"reflect"
	"strconv"
)

// Options 用于定制 Decode 行为。
type Options struct {
	// 是否启用宽松解码（默认 true）：
	// 例如 "123" -> int、1.0 -> int64 等。
	WeaklyTypedInput bool
}

// DefaultOptions 返回默认选项。
func DefaultOptions() Options {
	return Options{
		WeaklyTypedInput: true,
	}
}

// WithWeaklyTypedInput 便捷开关。
func WithWeaklyTypedInput(v bool) Options {
	return Options{WeaklyTypedInput: v}
}

// DecodeStruct 将 *structpb.Struct 动态解码到任意结构体 T。
// T 通常是你定义的业务负载，例如 AuthPayload / VotePayload 等。
// 结构体字段读取使用 `json` tag。
func DecodeStruct[T any](st *structpb.Struct, opts ...Options) (*T, error) {
	if st == nil {
		return nil, fmt.Errorf("struct is nil")
	}

	// 解析选项
	cfg := DefaultOptions()
	if len(opts) > 0 {
		cfg = opts[0]
	}

	m := st.AsMap()
	var out T

	decCfg := &mapstructure.DecoderConfig{
		TagName:          "json",
		Result:           &out,
		WeaklyTypedInput: cfg.WeaklyTypedInput,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			floatToIntHook(),
			sliceAnyToSliceStringHook(),
			jsonRawStringToMapHook(),
		),
	}

	dec, err := mapstructure.NewDecoder(decCfg)
	if err != nil {
		return nil, fmt.Errorf("new decoder: %w", err)
	}

	if err := dec.Decode(m); err != nil {
		return nil, fmt.Errorf("decode struct: %w", err)
	}
	return &out, nil
}

// -----------------------------
// 基础读取工具（动态场景常用）
// -----------------------------

// ReadString 从 Struct 中读取 string 字段。
func ReadString(st *structpb.Struct, key string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("struct is nil")
	}
	v, ok := st.AsMap()[key]
	if !ok || v == nil {
		return "", fmt.Errorf("missing field %q", key)
	}
	switch t := v.(type) {
	case string:
		return t, nil
	default:
		return "", fmt.Errorf("field %q not string (got %T)", key, v)
	}
}

// ReadInt64 从 Struct 中读取整数（兼容 float64 / int / string 数字）。
func ReadInt64(st *structpb.Struct, key string) (int64, error) {
	if st == nil {
		return 0, fmt.Errorf("struct is nil")
	}
	v, ok := st.AsMap()[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("missing field %q", key)
	}
	switch t := v.(type) {
	case float64:
		return int64(t), nil
	case int64:
		return t, nil
	case int32:
		return int64(t), nil
	case int:
		return int64(t), nil
	case json.Number:
		return t.Int64()
	case string:
		// 尝试把数字字符串转成 int64
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("field %q string parse int64: %w", key, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("field %q type %T not number", key, v)
	}
}

// ReadStringSlice 从 Struct 中读取字符串数组（兼容 []any）。
func ReadStringSlice(st *structpb.Struct, key string) ([]string, error) {
	if st == nil {
		return nil, fmt.Errorf("struct is nil")
	}
	v, ok := st.AsMap()[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("missing field %q", key)
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("field %q type %T not array", key, v)
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		switch s := it.(type) {
		case string:
			out = append(out, s)
		case json.Number:
			out = append(out, s.String())
		default:
			// 兜底：JSON 化
			b, _ := json.Marshal(s)
			out = append(out, string(b))
		}
	}
	return out, nil
}

// -----------------------------
// Decode Hooks
// -----------------------------

// floatToIntHook：把 float64 自动转为 int / int32 / int64。
func floatToIntHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Kind, data any) (any, error) {
		if from != reflect.Float64 {
			return data, nil
		}
		switch to {
		case reflect.Int:
			return int(data.(float64)), nil
		case reflect.Int32:
			return int32(data.(float64)), nil
		case reflect.Int64:
			return int64(data.(float64)), nil
		}
		return data, nil
	}
}

// sliceAnyToSliceStringHook：把 []any 自动转为 []string。
func sliceAnyToSliceStringHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Kind, data any) (any, error) {
		if from != reflect.Slice || to != reflect.Slice {
			return data, nil
		}
		src, ok := data.([]any)
		if !ok {
			return data, nil
		}
		// 目标是否是 []string
		// 无法直接判断目标泛型，此处用运行时转换。
		out := make([]string, 0, len(src))
		for _, it := range src {
			switch v := it.(type) {
			case string:
				out = append(out, v)
			case json.Number:
				out = append(out, v.String())
			default:
				b, _ := json.Marshal(v)
				out = append(out, string(b))
			}
		}
		return out, nil
	}
}

// jsonRawStringToMapHook：把 JSON 字符串自动转为 map[string]any（用于某些嵌套字符串 JSON 字段）。
func jsonRawStringToMapHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Kind, data any) (any, error) {
		if from != reflect.String || to != reflect.Map {
			return data, nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(data.(string)), &m); err == nil {
			return m, nil
		}
		return data, nil
	}
}
