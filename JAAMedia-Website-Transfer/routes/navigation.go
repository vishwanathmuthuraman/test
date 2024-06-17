package routes

type Route struct {
	Name string
	Path string
}

func RbacNavigation(account *WebUserAccount) map[string][]Route {

	if account == nil {
		return map[string][]Route{}
	}

	if account.Type == "admin" {
		return map[string][]Route{
			"Management": {
				Route{"Dashboard", "/admin/dashboard"},
				Route{"Sponsored Videos", "/admin/videos?sponsored_only=true"},
				Route{"Non-Sponsored Videos", "/admin/videos?non_sponsored_only=true"},
				Route{"Strategies", "/strategies"},
				Route{"Tracking Status", "/admin/tracking_status"},
				Route{"Retention Data", "/admin/retention"},
				Route{"Finances", "/admin/finances"},
				Route{"Creativity", "/admin/creativity?n_days=10"},
			},
			"Users": {
				Route{"Virtual Assistants", "/admin/manage_va"},
				Route{"Sponsors", "/admin/manage_sponsors"},
				Route{"Writers", "/admin/manage_writers"},
			},
			"Data Entry": {
				Route{"Add Video Only", "/express_entry"},
				Route{"Update Video Details", "/admin/videos"},
				Route{"Bulk Entry / CSV", "/upload_csv"},
				Route{"Upload Retention", "/retention"},
			},
			"System": {
				Route{"Log Out", "/logout"},
			},
		}
	} else if account.Type == "sponsor" {
		return map[string][]Route{
			"Navigation": {
				Route{"Dashboard", "/sponsor/dash"},
			},
			"System": {
				Route{"Log Out", "/logout"},
			},
		}
	} else if account.Type == "va" {
		return map[string][]Route{
			"Data Entry": {
				Route{"Enter One Video", "/express_entry"},
				Route{"Update Video Details", "/admin/videos"},
				Route{"Bulk Upload (CSV)", "/upload_csv"},
			},

			"System": {
				Route{"Log Out", "/logout"},
			},
		}
	}

	return map[string][]Route{}
}
