package config

import (
	"encoding"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// field is one setting: a leaf of Settings and its key.
type field struct {
	key string
	v   reflect.Value
}

var textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()

// fields lists the settings of s in field order. A struct field that does
// not read itself from text is a group: its fields are settings too.
func fields(s *Settings) []field {
	var out []field
	var walk func(v reflect.Value, prefix string)
	walk = func(v reflect.Value, prefix string) {
		for i := range v.NumField() {
			f, fv := v.Type().Field(i), v.Field(i)
			key := prefix + "_" + strings.ToUpper(f.Name)
			if f.Type.Kind() == reflect.Struct && !reflect.PointerTo(f.Type).Implements(textUnmarshaler) {
				walk(fv, key)
				continue
			}
			out = append(out, field{key, fv})
		}
	}
	walk(reflect.ValueOf(s).Elem(), Prefix)
	return out
}

// keyOf returns the key of the field of s that p points to.
func keyOf(s *Settings, p any) string {
	for _, f := range fields(s) {
		if f.v.Addr().Interface() == p {
			return f.key
		}
	}
	panic(fmt.Sprintf("config: %T is not a field of Settings", p))
}

// set parses raw into the field. Errors leave the value out: it may be a
// secret.
func (f field) set(raw string) error {
	switch p := f.v.Addr().Interface().(type) {
	case *string:
		*p = raw
	case *int:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return errors.New("must be a whole number")
		}
		*p = n
	case *bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return errors.New("must be true or false")
		}
		*p = b
	case *time.Duration:
		d, err := time.ParseDuration(raw)
		if err != nil {
			return errors.New("must be a duration, e.g. 10s, 5m or 1h30m")
		}
		*p = d
	case **time.Location:
		loc, err := time.LoadLocation(raw)
		if err != nil {
			return fmt.Errorf("unknown time zone %q", raw)
		}
		*p = loc
	case encoding.TextUnmarshaler:
		return p.UnmarshalText([]byte(raw))
	default:
		panic(fmt.Sprintf("config: %s has a type the loader cannot read: %s", f.key, f.v.Type()))
	}
	return nil
}

// LogFormat is how logs are written.
type LogFormat string

// The log formats.
const (
	LogText LogFormat = "text"
	LogJSON LogFormat = "json"
)

// UnmarshalText reads text or json, in any case.
func (l *LogFormat) UnmarshalText(b []byte) error {
	switch v := LogFormat(strings.ToLower(string(b))); v {
	case LogText, LogJSON:
		*l = v
		return nil
	}
	return errors.New(`must be "text" or "json"`)
}

// Proxies are IP ranges, written as ParseTrustedProxies reads them.
type Proxies []netip.Prefix

// UnmarshalText reads a list of addresses and CIDR ranges.
func (p *Proxies) UnmarshalText(b []byte) error {
	v, err := ParseTrustedProxies(string(b))
	if err != nil {
		return err
	}
	*p = v
	return nil
}

// ByteSize is a size in bytes.
type ByteSize int64

var byteUnits = map[string]int64{
	"": 1, "b": 1,
	"kb": 1e3, "mb": 1e6, "gb": 1e9, "tb": 1e12,
	"kib": 1 << 10, "mib": 1 << 20, "gib": 1 << 30, "tib": 1 << 40,
}

// UnmarshalText reads a whole number of bytes, optionally followed by a
// unit: KB, MB, GB, TB (powers of 1000) or KiB, MiB, GiB, TiB (of 1024).
func (b *ByteSize) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	i := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if i < 0 {
		i = len(s)
	}
	n, err := strconv.ParseInt(s[:i], 10, 64)
	unit, ok := byteUnits[strings.ToLower(strings.TrimSpace(s[i:]))]
	if err != nil || !ok || n > math.MaxInt64/unit {
		return errors.New("must be a size, e.g. 100MB or 100MiB")
	}
	*b = ByteSize(n * unit)
	return nil
}
