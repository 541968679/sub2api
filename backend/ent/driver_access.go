package ent

import "entgo.io/ent/dialect"

// Driver exposes the underlying driver for integration code that needs raw SQL.
func (c *Client) Driver() dialect.Driver {
	return c.driver
}
