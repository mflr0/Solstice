package devseed

import (
	"net"
	"strconv"
)

// ensureMail points e-mail at Mailpit when no mail server is set.
func ensureMail(admin *client, hostPort string) (bool, error) {
	var cur struct{ Host string }
	if err := admin.call("GET", "/api/admin/mail", nil, &cur); err != nil {
		return false, err
	}
	if cur.Host != "" {
		return false, nil
	}
	host, rawPort, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return false, err
	}
	return true, admin.call("PUT", "/api/admin/mail", map[string]any{"enabled": true, "host": host, "port": port,
		"security": "none", "username": "", "password": nil, "from": "solstice@example.org", "fromName": "Solstice"}, nil)
}
