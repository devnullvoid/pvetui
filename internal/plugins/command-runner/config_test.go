package commandrunner

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid timeout - zero",
			config: Config{
				Timeout:       0,
				MaxOutputSize: 1024,
			},
			wantErr: true,
		},
		{
			name: "invalid timeout - negative",
			config: Config{
				Timeout:       -1 * time.Second,
				MaxOutputSize: 1024,
			},
			wantErr: true,
		},
		{
			name: "invalid max output size - zero",
			config: Config{
				Timeout:       30 * time.Second,
				MaxOutputSize: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid max output size - negative",
			config: Config{
				Timeout:       30 * time.Second,
				MaxOutputSize: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name       string
		cmd        string
		wantParams []string
	}{
		{
			name:       "no parameters",
			cmd:        "uptime",
			wantParams: nil,
		},
		{
			name:       "single parameter",
			cmd:        "systemctl status {service}",
			wantParams: []string{"service"},
		},
		{
			name:       "multiple parameters",
			cmd:        "journalctl -u {service} -n {lines}",
			wantParams: []string{"service", "lines"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := ParseTemplate(tt.cmd)
			if len(template.Parameters) != len(tt.wantParams) {
				t.Errorf("ParseTemplate() got %d params, want %d", len(template.Parameters), len(tt.wantParams))
				return
			}
			for i, param := range template.Parameters {
				if param != tt.wantParams[i] {
					t.Errorf("ParseTemplate() param[%d] = %v, want %v", i, param, tt.wantParams[i])
				}
			}
		})
	}
}

func TestCommandTemplate_FillTemplate(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		values     map[string]string
		wantResult string
		wantErr    bool
	}{
		{
			name:       "simple parameter",
			template:   "systemctl status {service}",
			values:     map[string]string{"service": "nginx"},
			wantResult: "systemctl status nginx",
			wantErr:    false,
		},
		{
			name:       "multiple parameters",
			template:   "journalctl -u {service} -n {lines}",
			values:     map[string]string{"service": "nginx", "lines": "50"},
			wantResult: "journalctl -u nginx -n 50",
			wantErr:    false,
		},
		{
			name:       "missing parameter",
			template:   "systemctl status {service}",
			values:     map[string]string{},
			wantResult: "",
			wantErr:    true,
		},
		{
			name:       "shell metacharacters - should fail",
			template:   "systemctl status {service}",
			values:     map[string]string{"service": "nginx; rm -rf /"},
			wantResult: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := ParseTemplate(tt.template)
			result, err := tmpl.FillTemplate(tt.values)
			if (err != nil) != tt.wantErr {
				t.Errorf("FillTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.wantResult {
				t.Errorf("FillTemplate() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestContainsShellMetachars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "safe string",
			input: "nginx",
			want:  false,
		},
		{
			name:  "safe string with dash",
			input: "my-service",
			want:  false,
		},
		{
			name:  "dangerous - semicolon",
			input: "nginx; rm -rf /",
			want:  true,
		},
		{
			name:  "dangerous - pipe",
			input: "nginx | cat",
			want:  true,
		},
		{
			name:  "dangerous - backtick",
			input: "nginx`whoami`",
			want:  true,
		},
		{
			name:  "dangerous - dollar",
			input: "nginx$(whoami)",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsShellMetachars(tt.input); got != tt.want {
				t.Errorf("containsShellMetachars() = %v, want %v", got, tt.want)
			}
		})
	}
}
