package api

// MockConfig implements interfaces.Config for testing purposes.
type MockConfig struct {
	Addr        string
	User        string
	Password    string
	Realm       string
	TokenID     string
	TokenSecret string
	Insecure    bool
}

func (c *MockConfig) GetAddr() string {
	return c.Addr
}

func (c *MockConfig) GetUser() string {
	return c.User
}

func (c *MockConfig) GetPassword() string {
	return c.Password
}

func (c *MockConfig) GetRealm() string {
	return c.Realm
}

func (c *MockConfig) GetTokenID() string {
	return c.TokenID
}

func (c *MockConfig) GetTokenSecret() string {
	return c.TokenSecret
}

func (c *MockConfig) GetInsecure() bool {
	return c.Insecure
}

func (c *MockConfig) IsUsingTokenAuth() bool {
	return c.TokenID != "" && c.TokenSecret != ""
}

func (c *MockConfig) GetAPIToken() string {
	if c.IsUsingTokenAuth() {
		return "PVEAPIToken=" + c.User + "@" + c.Realm + "!" + c.TokenID + "=" + c.TokenSecret
	}
	return ""
}
