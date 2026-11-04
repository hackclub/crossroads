package db

// Address is a mailing address, as stored in users.address and
// orders.shipping_address.
type Address struct {
	Name       string `json:"name"`
	Line1      string `json:"line_1"`
	Line2      string `json:"line_2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}
