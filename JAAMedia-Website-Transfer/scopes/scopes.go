package scopes

type Scope string

const (
	BulkEntry      Scope = "bulk_entry"
	ExpressEntry   Scope = "express_entry"
	DetailedEntry  Scope = "detailed_entry"
	ManageVa       Scope = "manage_va"
	ManageSponsors Scope = "manage_sponsors"
	ManageWriters  Scope = "manage_writers"
	EnterIncome    Scope = "enter_income"
	EnterExpenses  Scope = "enter_expenses"
)
