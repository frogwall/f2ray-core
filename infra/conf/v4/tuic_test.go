package v4_test

import (
	"encoding/json"
	"testing"

	v4 "github.com/frogwall/f2ray-core/v5/infra/conf/v4"
	"github.com/frogwall/f2ray-core/v5/proxy/tuic"
	"github.com/stretchr/testify/assert"
)

func TestTUICConfigWithStreamSettings(t *testing.T) {
	jsonConfig := `{
		"protocol": "tuic",
		"settings": {
			"servers": [
				{
					"address": "127.0.0.1",
					"port": 8443,
					"uuid": "uuid",
					"password": "password"
				}
			]
		},
		"streamSettings": {
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"allowInsecure": true,
				"alpn": ["h3"]
			}
		}
	}`

	config := new(v4.OutboundDetourConfig)
	err := json.Unmarshal([]byte(jsonConfig), config)
	assert.NoError(t, err)

	outboundConfig, err := config.Build()
	assert.NoError(t, err)

	// Unpack ProxySettings to check TUIC config
	tuicConfig := new(tuic.ClientConfig)
	err = outboundConfig.ProxySettings.UnmarshalTo(tuicConfig)
	assert.NoError(t, err)

	assert.NotNil(t, tuicConfig.Tls)
	assert.Equal(t, "example.com", tuicConfig.Tls.ServerName)
	assert.True(t, tuicConfig.Tls.AllowInsecure)
	assert.Equal(t, []string{"h3"}, tuicConfig.Tls.Alpn)
}
