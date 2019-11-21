package testdata

type Person struct {
	ID string `json:"id"`
}

type Account struct {
	ID     string `json:"id"`
	PlanID string `json:"plan"`
}

type User struct {
	ID        string `json:"id"`
	PersonID  string `json:"person"`
	AccountID string `json:"account"`
	Type      string `json:"type"`
}

type Plan struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var People = map[string]Person{
	"1": {
		ID: "1",
	},
	"2": {
		ID: "2",
	},
}

var Accounts = map[string]Account{
	"123": {
		ID:     "123",
		PlanID: "1",
	},
	"234": {
		ID:     "234",
		PlanID: "2",
	},
}

var Users = map[string]User{
	"1": {
		ID:        "1",
		PersonID:  "1",
		AccountID: "123",
		Type:      "ACCOUNT_HOLDER",
	},
	"2": {
		ID:        "2",
		PersonID:  "1",
		AccountID: "234",
	},
	"3": {
		ID:        "3",
		PersonID:  "2",
		AccountID: "123",
		Type:      "EMPLOYEE",
	},
}

var Plans = map[string]Plan{
	"1": {
		ID:   "1",
		Name: "Basic (20 Employees)",
	},
	"2": {
		ID:   "2",
		Name: "Standard (30 Employees)",
	},
}
