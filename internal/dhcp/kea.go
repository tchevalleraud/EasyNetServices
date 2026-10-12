package dhcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// Result codes of the Kea control channel.
const (
	keaSuccess     = 0
	keaEmpty       = 3
	commandTimeout = 10 * time.Second
)

var errKeaEmpty = errors.New("empty result")

// keaClient sends commands to the control socket of a Kea server.
type keaClient struct {
	socket string
}

type keaResponse struct {
	Result    int             `json:"result"`
	Text      string          `json:"text"`
	Arguments json.RawMessage `json:"arguments"`
}

// command sends a command and decodes its arguments into out. It returns
// errKeaEmpty when Kea found nothing, e.g. an unknown lease.
func (c *keaClient) command(ctx context.Context, name string, args, out any) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	req := map[string]any{"command": name}
	if args != nil {
		req["arguments"] = args
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var resp keaResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("%s: invalid response: %w", name, err)
	}
	switch resp.Result {
	case keaSuccess:
	case keaEmpty:
		return errKeaEmpty
	default:
		return fmt.Errorf("%s: %s", name, resp.Text)
	}
	if out != nil && len(resp.Arguments) > 0 {
		return json.Unmarshal(resp.Arguments, out)
	}
	return nil
}
