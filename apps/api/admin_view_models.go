package main

import (
	"html/template"
	"sync"
	"time"
)

const (
	adminLanguageCookieName        = "zwerffiets_admin_language"
	adminDefaultLanguage           = "nl"
	adminLanguageCookieMaxAge      = 180 * 24 * time.Hour
	adminDisplayTimestampLayout    = "2006-01-02 15:04"
	adminTemplateLoginPath         = "templates/admin/login.tmpl"
	adminTemplateTriagePath        = "templates/admin/triage.tmpl"
	adminTemplateReportPath        = "templates/admin/report_detail.tmpl"
	adminTemplateMapPath           = "templates/admin/map.tmpl"
	adminTemplateExportsPath       = "templates/admin/exports.tmpl"
	adminSignalClassNone           = "signal-none"
	adminSignalClassWeak           = "signal-weak"
	adminSignalClassStrong         = "signal-strong"
	adminStatusActionLabelTemplate = "action_mark_%s"
)

var (
	adminTranslations = map[string]map[string]string{
		"nl": {
			"app_title":                              "ZwerfFiets Beheer",
			"workspace_signed_in":                    "Ingelogd als",
			"language_label":                         "Taal",
			"language_apply":                         "Wijzigen",
			"language_nl":                            "Nederlands",
			"language_en":                            "English",
			"nav_triage":                             "Triage",
			"nav_map":                                "Kaart",
			"nav_exports":                            "Export",
			"nav_logout":                             "Uitloggen",
			"page_title_login":                       "Beheer login",
			"page_title_triage":                      "Triage",
			"page_title_report":                      "Melding",
			"page_title_map":                         "Kaart",
			"page_title_exports":                     "Export",
			"login_title":                            "Operator login",
			"login_hint":                             "Log in met je operator-account.",
			"login_email":                            "E-mail",
			"login_password":                         "Wachtwoord",
			"login_button":                           "Inloggen",
			"error_invalid_credentials":              "Ongeldige inloggegevens.",
			"error_login_failed":                     "Inloggen is mislukt.",
			"error_reports_load_failed":              "Meldingen laden is mislukt.",
			"error_report_load_failed":               "Melding laden is mislukt.",
			"error_status_update_failed":             "Status bijwerken is mislukt.",
			"error_merge_failed":                     "Samenvoegen is mislukt.",
			"error_export_load_failed":               "Exporten laden is mislukt.",
			"error_export_generate_failed":           "Export genereren is mislukt.",
			"error_merge_input_required":             "Vul minimaal één duplicaat-id in.",
			"notice_status_updated":                  "Status bijgewerkt.",
			"notice_merge_completed":                 "Duplicaten samengevoegd.",
			"notice_export_generated":                "Export gegenereerd.",
			"filter_status":                          "Status",
			"filter_city":                            "Stad/Plaats",
			"filter_signal":                          "Signaal",
			"filter_qualifying":                      "Geldige herbevestiging",
			"filter_strong_only":                     "Alleen sterk",
			"filter_sort":                            "Sortering",
			"filter_all":                             "Alle",
			"filter_yes":                             "Ja",
			"filter_no":                              "Nee",
			"filter_apply":                           "Toepassen",
			"sort_newest":                            "Nieuwste",
			"sort_signal":                            "Signaal",
			"col_public_id":                          "Publieke ID",
			"col_photo":                              "Foto",
			"col_status":                             "Status",
			"col_city":                               "Stad/Plaats",
			"col_signal":                             "Signaal",
			"col_unique_reporters":                   "Unieke melders",
			"col_last_reconfirmed":                   "Laatste herbevestiging",
			"col_created":                            "Aangemaakt",
			"col_tags":                               "Tags",
			"col_actions":                            "Acties",
			"bulk_selected":                          "geselecteerd",
			"bulk_choose_action":                     "Kies actie…",
			"bulk_apply":                             "Toepassen",
			"error_bulk_no_selection":                "Selecteer meldingen en kies een actie.",
			"error_bulk_partial":                     "%d van %d meldingen bijgewerkt (sommige zijn mislukt).",
			"notice_bulk_updated":                    "%d meldingen bijgewerkt.",
			"photo_missing":                          "Geen foto",
			"triage_empty":                           "Geen meldingen voor deze filters.",
			"report_back":                            "Terug naar triage",
			"report_title":                           "Melding",
			"report_location":                        "Locatie",
			"report_tags":                            "Tags",
			"report_note":                            "Notitie",
			"report_photos":                          "Foto's",
			"report_merge_title":                     "Duplicaten samenvoegen",
			"report_merge_hint":                      "Voer komma-gescheiden melding-IDs in.",
			"report_merge_label":                     "Duplicaat IDs",
			"report_merge_button":                    "Samenvoegen",
			"report_address":                         "Adres",
			"report_city":                            "Stad/Plaats",
			"report_signal_title":                    "Signaal",
			"report_signal_strength":                 "Sterkte",
			"report_signal_group":                    "Bike group",
			"report_signal_total_reports":            "Totaal meldingen",
			"report_signal_unique_reporters":         "Unieke melders",
			"report_signal_same_reconfirmations":     "Herbevestigingen zelfde melder",
			"report_signal_distinct_reconfirmations": "Herbevestigingen andere melder",
			"report_signal_first_qualifying":         "Eerste geldige herbevestiging",
			"report_signal_last_qualifying":          "Laatste geldige herbevestiging",
			"report_signal_timeline":                 "Signaal timeline",
			"report_events_timeline":                 "Evenementen timeline",
			"map_title":                              "Meldingenkaart",
			"map_description":                        "Clusterweergave van meldingen.",
			"exports_title":                          "Export batches",
			"exports_period":                         "Periode",
			"exports_generate":                       "Genereer export",
			"exports_weekly":                         "Wekelijks",
			"exports_monthly":                        "Maandelijks",
			"exports_all_time":                       "Alles (Totaal)",
			"exports_col_generated":                  "Gegenereerd",
			"exports_col_type":                       "Type",
			"exports_col_period":                     "Periode",
			"exports_col_rows":                       "Rijen",
			"exports_col_files":                      "Bestanden",
			"common_dash":                            "-",
			"status_new":                             "Nieuw",
			"status_triaged":                         "Getrieerd",
			"status_forwarded":                       "Doorgestuurd",
			"status_resolved":                        "Opgelost",
			"status_invalid":                         "Ongeldig",
			"action_mark_triaged":                    "Markeer getrieerd",
			"action_mark_forwarded":                  "Markeer doorgestuurd",
			"action_mark_resolved":                   "Markeer opgelost",
			"action_mark_invalid":                    "Markeer ongeldig",
			"action_apply":                           "Toepassen",
			"signal_none":                            "Geen",
			"signal_weak_same_reporter":              "Zwak (zelfde melder)",
			"signal_strong_distinct_reporters":       "Sterk (andere melders)",
			"event_created":                          "Aangemaakt",
			"event_status_changed":                   "Status gewijzigd",
			"event_merged":                           "Samengevoegd",
			"event_exported":                         "Geëxporteerd",
			"event_signal_reconfirmation_counted":    "Herbevestiging geteld",
			"event_signal_reconfirmation_ignored_same_day": "Herbevestiging genegeerd (zelfde dag)",
			"event_signal_strength_changed":                "Signaalsterkte gewijzigd",
			"timeline_ignored":                             "Genegeerd (zelfde dag)",
			"timeline_same":                                "Geteld (zelfde melder)",
			"timeline_distinct":                            "Geteld (andere melder)",
			"timeline_recorded":                            "Geregistreerd",

			"nav_operators":                  "Operators",
			"page_title_operators":           "Operators",
			"page_title_operator_new":        "Nieuwe operator",
			"operator_create_button":         "Nieuwe operator",
			"operator_email":                 "E-mail",
			"operator_role":                  "Rol",
			"operator_municipality":          "Gemeente",
			"operator_password":              "Wachtwoord",
			"operator_status":                "Status",
			"operator_save":                  "Opslaan",
			"role_admin":                     "Beheerder",
			"role_municipality_operator":     "Gemeente operator",
			"status_active":                  "Actief",
			"status_inactive":                "Inactief",
			"action_activate":                "Activeren",
			"action_deactivate":              "Deactiveren",
			"ui_cancel":                      "Annuleren",
			"error_operator_create_failed":   "Aanmaken mislukt.",
			"notice_operator_created":        "Operator aangemaakt.",
			"error_operator_toggle_failed":   "Status wijzigen mislukt.",
			"notice_status_toggled":          "Status gewijzigd.",
			"error_invalid_municipality":     "Ongeldige gemeente geselecteerd.",
			"page_title_operator_edit":       "Operator bewerken",
			"notice_operator_updated":        "Operator bijgewerkt.",
			"error_operator_update_failed":   "Bijwerken mislukt.",
			"operator_password_hint":         "Leeg laten om niet te wijzigen.",
			"page_title_report_edit":         "Melding bewerken",
			"report_municipality":            "Gemeente",
			"report_postcode":                "Postcode",
			"report_lat":                     "Breedtegraad",
			"report_lng":                     "Lengtegraad",
			"report_edit_button":             "Bewerken",
			"report_edit_save":               "Opslaan",
			"notice_report_updated":          "Melding bijgewerkt.",
			"error_report_update_failed":     "Melding bijwerken is mislukt.",
			"action_disable_reports":         "Berichten uitschakelen",
			"action_enable_reports":          "Berichten inschakelen",
			"operator_receives_reports":      "Ontvangt meldingen",
			"operator_unsubscribe_requested": "Afmelding verzocht",
			"status_yes":                     "Ja",
			"status_no":                      "Nee",
			"badge_unsubscribe_requested":    "Verzocht",
			"nav_users":                      "Gebruikers",
			"page_title_users":               "Gebruikers",
			"placeholder_search_users":       "Zoek mail...",
			"page_title_user_edit":           "Gebruiker aanpassen",
			"col_user_email":                 "E-mail",
			"col_user_status":                "Status",
			"col_user_created":               "Geregistreerd",
			"user_save":                      "Opslaan",
			"notice_user_updated":            "Gebruiker bijgewerkt.",
			"error_user_update_failed":       "Bijwerken mislukt.",
			"error_users_load_failed":        "Laden gebruikers mislukt.",
			"error_user_not_found":           "Gebruiker niet gevonden.",
			"notice_bulk_deleted":            "%d gebruikers verwijderd.",
			"error_bulk_operation_failed":    "Bulk-bewerking mislukt.",
			"user_confirm_delete":            "Weet je zeker dat je de geselecteerde gebruikers wilt verwijderen? Dit kan niet ongedaan worden gemaakt.",
			"action_delete":                  "Verwijderen",
		},
		"en": {
			"app_title":                              "ZwerfFiets Admin",
			"workspace_signed_in":                    "Signed in as",
			"language_label":                         "Language",
			"language_apply":                         "Apply",
			"language_nl":                            "Nederlands",
			"language_en":                            "English",
			"nav_triage":                             "Triage",
			"nav_map":                                "Map",
			"nav_exports":                            "Exports",
			"nav_logout":                             "Logout",
			"page_title_login":                       "Admin login",
			"page_title_triage":                      "Triage",
			"page_title_report":                      "Report",
			"page_title_map":                         "Map",
			"page_title_exports":                     "Exports",
			"login_title":                            "Operator login",
			"login_hint":                             "Sign in with your operator account.",
			"login_email":                            "Email",
			"login_password":                         "Password",
			"login_button":                           "Sign in",
			"error_invalid_credentials":              "Invalid credentials.",
			"error_login_failed":                     "Sign in failed.",
			"error_reports_load_failed":              "Failed to load reports.",
			"error_report_load_failed":               "Failed to load report details.",
			"error_status_update_failed":             "Failed to update status.",
			"error_merge_failed":                     "Failed to merge reports.",
			"error_export_load_failed":               "Failed to load exports.",
			"error_export_generate_failed":           "Failed to generate export.",
			"error_merge_input_required":             "Enter at least one duplicate report ID.",
			"notice_status_updated":                  "Status updated.",
			"notice_merge_completed":                 "Duplicate merge completed.",
			"notice_export_generated":                "Export generated.",
			"filter_status":                          "Status",
			"filter_city":                            "City/Town",
			"filter_signal":                          "Signal",
			"filter_qualifying":                      "Qualifying reconfirmation",
			"filter_strong_only":                     "Strong only",
			"filter_sort":                            "Sort",
			"filter_all":                             "All",
			"filter_yes":                             "Yes",
			"filter_no":                              "No",
			"filter_apply":                           "Apply",
			"sort_newest":                            "Newest",
			"sort_signal":                            "Signal",
			"col_public_id":                          "Public ID",
			"col_photo":                              "Photo",
			"col_status":                             "Status",
			"col_city":                               "City/Town",
			"col_signal":                             "Signal",
			"col_unique_reporters":                   "Unique reporters",
			"col_last_reconfirmed":                   "Last reconfirmed",
			"col_created":                            "Created",
			"col_tags":                               "Tags",
			"col_actions":                            "Actions",
			"bulk_selected":                          "selected",
			"bulk_choose_action":                     "Choose action…",
			"bulk_apply":                             "Apply",
			"error_bulk_no_selection":                "Select reports and choose an action.",
			"error_bulk_partial":                     "%d of %d reports updated (some failed).",
			"notice_bulk_updated":                    "%d reports updated.",
			"photo_missing":                          "No photo",
			"triage_empty":                           "No reports for current filters.",
			"report_back":                            "Back to triage",
			"report_title":                           "Report",
			"report_location":                        "Location",
			"report_tags":                            "Tags",
			"report_note":                            "Note",
			"report_photos":                          "Photos",
			"report_merge_title":                     "Merge duplicates",
			"report_merge_hint":                      "Enter comma-separated report IDs.",
			"report_merge_label":                     "Duplicate IDs",
			"report_merge_button":                    "Merge",
			"report_address":                         "Address",
			"report_city":                            "City/Town",
			"report_signal_title":                    "Signal",
			"report_signal_strength":                 "Strength",
			"report_signal_group":                    "Bike group",
			"report_signal_total_reports":            "Total reports",
			"report_signal_unique_reporters":         "Unique reporters",
			"report_signal_same_reconfirmations":     "Same reporter reconfirmations",
			"report_signal_distinct_reconfirmations": "Distinct reporter reconfirmations",
			"report_signal_first_qualifying":         "First qualifying reconfirmation",
			"report_signal_last_qualifying":          "Last qualifying reconfirmation",
			"report_signal_timeline":                 "Signal timeline",
			"report_events_timeline":                 "Event timeline",
			"map_title":                              "Report map",
			"map_description":                        "Clustered map of report locations.",
			"exports_title":                          "Export batches",
			"exports_period":                         "Period",
			"exports_generate":                       "Generate export",
			"exports_weekly":                         "Weekly",
			"exports_monthly":                        "Monthly",
			"exports_col_generated":                  "Generated",
			"exports_col_type":                       "Type",
			"exports_col_period":                     "Period",
			"exports_col_rows":                       "Rows",
			"exports_col_files":                      "Files",
			"common_dash":                            "-",
			"status_new":                             "New",
			"status_triaged":                         "Triaged",
			"status_forwarded":                       "Forwarded",
			"status_resolved":                        "Resolved",
			"status_invalid":                         "Invalid",
			"action_mark_triaged":                    "Mark triaged",
			"action_mark_forwarded":                  "Mark forwarded",
			"action_mark_resolved":                   "Mark resolved",
			"action_mark_invalid":                    "Mark invalid",
			"action_apply":                           "Apply",
			"signal_none":                            "None",
			"signal_weak_same_reporter":              "Weak (same reporter)",
			"signal_strong_distinct_reporters":       "Strong (distinct reporters)",
			"event_created":                          "Created",
			"event_status_changed":                   "Status changed",
			"event_merged":                           "Merged",
			"event_exported":                         "Exported",
			"event_signal_reconfirmation_counted":    "Reconfirmation counted",
			"event_signal_reconfirmation_ignored_same_day": "Reconfirmation ignored (same day)",
			"event_signal_strength_changed":                "Signal strength changed",
			"timeline_ignored":                             "Ignored (same day)",
			"timeline_same":                                "Counted (same reporter)",
			"timeline_distinct":                            "Counted (distinct reporter)",
			"timeline_recorded":                            "Recorded",

			"nav_operators":                  "Operators",
			"page_title_operators":           "Operators",
			"page_title_operator_new":        "New operator",
			"operator_create_button":         "New operator",
			"operator_email":                 "Email",
			"operator_role":                  "Role",
			"operator_municipality":          "Municipality",
			"operator_password":              "Password",
			"operator_status":                "Status",
			"operator_save":                  "Save",
			"role_admin":                     "Admin",
			"role_municipality_operator":     "Municipality operator",
			"status_active":                  "Active",
			"status_inactive":                "Inactive",
			"action_activate":                "Activate",
			"action_deactivate":              "Deactivate",
			"ui_cancel":                      "Cancel",
			"error_operator_create_failed":   "Failed to create operator.",
			"notice_operator_created":        "Operator created.",
			"error_operator_toggle_failed":   "Failed to toggle status.",
			"notice_status_toggled":          "Status toggled.",
			"error_invalid_municipality":     "Invalid municipality selected.",
			"page_title_operator_edit":       "Edit operator",
			"notice_operator_updated":        "Operator updated.",
			"error_operator_update_failed":   "Update failed.",
			"operator_password_hint":         "Leave blank to keep current password.",
			"page_title_report_edit":         "Edit report",
			"report_municipality":            "Municipality",
			"report_postcode":                "Postcode",
			"report_lat":                     "Latitude",
			"report_lng":                     "Longitude",
			"report_edit_button":             "Edit",
			"report_edit_save":               "Save",
			"notice_report_updated":          "Report updated.",
			"error_report_update_failed":     "Failed to update report.",
			"action_disable_reports":         "Disable reports",
			"action_enable_reports":          "Enable reports",
			"operator_receives_reports":      "Receives reports",
			"operator_unsubscribe_requested": "Unsubscribe requested",
			"status_yes":                     "Yes",
			"status_no":                      "No",
			"badge_unsubscribe_requested":    "Requested",
			"nav_users":                      "Users",
			"page_title_users":               "Users",
			"placeholder_search_users":       "Search email...",
			"page_title_user_edit":           "Edit user",
			"col_user_email":                 "Email",
			"col_user_status":                "Status",
			"col_user_created":               "Joined",
			"user_save":                      "Save",
			"notice_user_updated":            "User updated.",
			"error_user_update_failed":       "Update failed.",
			"error_users_load_failed":        "Failed to load users.",
			"error_user_not_found":           "User not found.",
			"notice_bulk_deleted":            "%d users deleted.",
			"error_bulk_operation_failed":    "Bulk operation failed.",
			"user_confirm_delete":            "Are you sure you want to delete the selected users? This action cannot be undone.",
			"action_delete":                  "Delete",
		},
	}

	adminTagLabels = map[string]map[string]string{
		"nl": {
			"flat_tires":             "Lekke banden",
			"rusted":                 "Verroest",
			"missing_parts":          "Onderdelen missen",
			"blocking_sidewalk":      "Blokkeert stoep",
			"damaged_frame":          "Beschadigd frame",
			"abandoned_long_time":    "Lang achtergelaten",
			"no_chain":               "Geen ketting",
			"wheel_missing":          "Wiel ontbreekt",
			"no_seat":                "Geen zadel",
			"other_visibility_issue": "Ander zichtbaar probleem",
		},
		"en": {
			"flat_tires":             "Flat tires",
			"rusted":                 "Rusted",
			"missing_parts":          "Missing parts",
			"blocking_sidewalk":      "Blocking sidewalk",
			"damaged_frame":          "Damaged frame",
			"abandoned_long_time":    "Abandoned for a long time",
			"no_chain":               "No chain",
			"wheel_missing":          "Wheel missing",
			"no_seat":                "No seat",
			"other_visibility_issue": "Other",
		},
	}

	adminTimeZoneOnce sync.Once
	adminTimeZone     *time.Location
)

type adminBaseViewData struct {
	Title           string
	Lang            string
	Text            map[string]string
	Session         *OperatorSession
	CurrentPath     string
	ActiveNav       string
	ErrorMessage    string
	NoticeMessage   string
	IncludeMapLibre bool
	ShowExports     bool
}

type adminLoginViewData struct {
	adminBaseViewData
	Email string
	Next  string
}

type adminStatusActionView struct {
	Status string
	Label  string
	Next   string
}

type adminReportRowView struct {
	ID                   int
	PublicID             string
	DetailURL            string
	PreviewPhotoURL      string
	StatusLabel          string
	City                 string
	SignalLabel          string
	SignalClass          string
	UniqueReporters      int
	LastReconfirmationAt string
	CreatedAt            string
	TagsLabel            string
	StatusActions        []adminStatusActionView
}

type adminReportFiltersView struct {
	City                        string
	Status                      string
	SignalStrength              string
	HasQualifyingReconfirmation string
	StrongOnly                  string
	Sort                        string
	CurrentURL                  string
}

type adminTriageViewData struct {
	adminBaseViewData
	Filters     adminReportFiltersView
	CityOptions []string
	Reports     []adminReportRowView
	Pagination  adminPaginationViewData
}

type adminPaginationViewData struct {
	CurrentPage   int
	TotalPages    int
	TotalCount    int
	NextPage      int
	PrevPage      int
	HasNext       bool
	HasPrev       bool
	PageURL       string
	PageSeparator string
}

type adminTimelineRowView struct {
	ReporterLabel string
	CreatedAt     string
	Description   string
}

type adminEventRowView struct {
	TypeLabel string
	Actor     string
	CreatedAt string
}

type adminSignalSummaryView struct {
	TotalReports                    int
	UniqueReporters                 int
	SameReporterReconfirmations     int
	DistinctReporterReconfirmations int
	FirstQualifying                 string
	LastQualifying                  string
}

type adminReportDetailViewData struct {
	adminBaseViewData
	ReportID            int
	PublicID            string
	BackURL             string
	ActionNext          string
	StatusLabel         string
	StatusActions       []adminStatusActionView
	Location            string
	Lat                 float64
	Lng                 float64
	TagsLabel           string
	NoteLabel           string
	Photos              []OperatorReportPhotoView
	MergeInput          string
	SignalStrengthLabel string
	BikeGroupID         int
	SignalSummary       adminSignalSummaryView
	Timeline            []adminTimelineRowView
	Events              []adminEventRowView
	Address             string
	City                string
	Municipality        string
	IsAdmin             bool
}

type adminReportEditViewData struct {
	adminBaseViewData
	ReportID       int
	BackURL        string
	Municipality   string
	Address        string
	Postcode       string
	Lat            float64
	Lng            float64
	Municipalities []string
}

type adminMapPoint struct {
	ID          int     `json:"id"`
	PublicID    string  `json:"publicId"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Status      string  `json:"status"`
	StatusLabel string  `json:"statusLabel"`
	Tags        string  `json:"tags"`
	Address     string  `json:"address"`
	City        string  `json:"city"`
}

type adminMapViewData struct {
	adminBaseViewData
	MapData template.JS
}

type adminExportRowView struct {
	ID                 int
	GeneratedAt        string
	PeriodType         string
	PeriodRange        string
	RowCount           int
	FilterStatus       string
	FilterMunicipality string
}

type adminExportsViewData struct {
	adminBaseViewData
	SelectedPeriod string
	Exports        []adminExportRowView
	Municipalities []string
	Statuses       []string
}

type adminReportFilters struct {
	City                        string
	Status                      string
	SignalStrength              string
	HasQualifyingReconfirmation string
	StrongOnly                  string
	Sort                        string
}
