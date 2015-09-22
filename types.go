package main

type cfgObject struct {
	Instagram struct {
		ClientID     string
		ClientSecret string
		TagName      string
		CallbackUrl  string
		Verify       string
	}
	Database struct {
		Host string
		Port string
		DB   string
	}
}
