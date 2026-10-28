package executil

import "errors"

// CertbotArgs constrains options and overrides hooks from cli.ini and renewals.
// Argument values may contain "hook"; only executable option names are restricted.
func CertbotArgs(input []string) ([]string, error) {
	if len(input) == 1 && input[0] == "--version" {
		return append([]string(nil), input...), nil
	}
	if len(input) == 0 || (input[0] != "certonly" && input[0] != "renew" && input[0] != "delete") {
		return nil, errors.New("Certbot command not allowed")
	}
	result := []string{input[0]}
	flags := map[string]bool{"--webroot": true, "--agree-tos": true, "--non-interactive": true, "--keep-until-expiring": true, "--no-directory-hooks": true}
	values := map[string]bool{"--webroot-path": true, "--domain": true, "--cert-name": true, "--email": true, "--server": true, "--config-dir": true, "--preferred-challenges": true}
	for i := 1; i < len(input); i++ {
		option := input[i]
		if flags[option] {
			if option != "--no-directory-hooks" {
				result = append(result, option)
			}
			continue
		}
		if i+1 >= len(input) {
			return nil, errors.New("missing Certbot option value")
		}
		value := input[i+1]
		i++
		if option == "--pre-hook" || option == "--post-hook" || option == "--deploy-hook" || option == "--renew-hook" {
			if value != "" {
				return nil, errors.New("Certbot hooks not allowed")
			}
			continue
		}
		if option == "--config" {
			if value != "/dev/null" {
				return nil, errors.New("external Certbot config not allowed")
			}
			continue
		}
		if !values[option] {
			return nil, errors.New("unsupported Certbot option")
		}
		result = append(result, option, value)
	}
	return append(result, "--config", "/dev/null", "--no-directory-hooks", "--pre-hook", "", "--post-hook", "", "--deploy-hook", ""), nil
}
