package api

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"solstice/internal/i18n"
)

// cleanName trims a name and checks its length.
func cleanName(name string, max int) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > max {
		return "", failf(http.StatusBadRequest, i18n.CommonNameRequired, max)
	}
	return name, nil
}

// pathID reads a numeric path value; a value that is not a number is 404.
func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, errNotFound
	}
	return id, nil
}
