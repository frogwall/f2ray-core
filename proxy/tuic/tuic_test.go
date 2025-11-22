package tuic_test

import (
	"testing"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/proxy/tuic"
	"github.com/stretchr/testify/assert"
)

func TestAccount(t *testing.T) {
	account := &tuic.Account{
		Uuid:     "FE35D05B-8803-45C4-BAE6-723AD2CD5D3D",
		Password: "test-password",
	}

	memAccount, err := account.AsAccount()
	common.Must(err)

	assert.NotNil(t, memAccount)
	tuicAccount, ok := memAccount.(*tuic.MemoryAccount)
	assert.True(t, ok)
	assert.Equal(t, "test-password", tuicAccount.Password)
	assert.Equal(t, "fe35d05b-8803-45c4-bae6-723ad2cd5d3d", tuicAccount.UUID.String())
}

func TestAccountEquals(t *testing.T) {
	account1 := &tuic.Account{
		Uuid:     "FE35D05B-8803-45C4-BAE6-723AD2CD5D3D",
		Password: "password1",
	}

	account2 := &tuic.Account{
		Uuid:     "FE35D05B-8803-45C4-BAE6-723AD2CD5D3D",
		Password: "password1",
	}

	account3 := &tuic.Account{
		Uuid:     "FE35D05B-8803-45C4-BAE6-723AD2CD5D3D",
		Password: "password2",
	}

	memAccount1, err := account1.AsAccount()
	common.Must(err)

	memAccount2, err := account2.AsAccount()
	common.Must(err)

	memAccount3, err := account3.AsAccount()
	common.Must(err)

	assert.True(t, memAccount1.Equals(memAccount2))
	assert.False(t, memAccount1.Equals(memAccount3))
}

func TestInvalidUUID(t *testing.T) {
	account := &tuic.Account{
		Uuid:     "invalid-uuid",
		Password: "test-password",
	}

	_, err := account.AsAccount()
	assert.Error(t, err)
}

func TestClientConfig(t *testing.T) {
	// Create a simple test to verify the config structure
	config := &tuic.ClientConfig{
		Server: []*protocol.ServerEndpoint{
			{
				Port: 8443,
			},
		},
		UdpRelayMode:      "native",
		CongestionControl: "bbr",
		ReduceRtt:         false,
		Tls: &tuic.TLSConfig{
			ServerName:    "example.com",
			Alpn:          []string{"h3"},
			AllowInsecure: true,
		},
	}

	assert.NotNil(t, config)
	assert.Equal(t, 1, len(config.Server))
	assert.Equal(t, uint32(8443), config.Server[0].Port)
	assert.Equal(t, "native", config.UdpRelayMode)
	assert.Equal(t, "bbr", config.CongestionControl)
	assert.False(t, config.ReduceRtt)
	assert.NotNil(t, config.Tls)
	assert.Equal(t, "example.com", config.Tls.ServerName)
	assert.Equal(t, []string{"h3"}, config.Tls.Alpn)
	assert.True(t, config.Tls.AllowInsecure)
}
