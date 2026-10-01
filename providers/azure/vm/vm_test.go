// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package vm_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/azure/vm"
)

// The field set follows Azure's instance metadata document at api-version
// 2021-12-13. Values are illustrative rather than captured. The document also
// carries osProfile.adminPassword, customData and userData, all of which this
// provider deliberately does not read.
const metadataJSON = `{
  "compute": {
    "vmId": "02aab8a4-74ef-476e-8182-f6d2ba4166a6",
    "location": "westeurope",
    "resourceId": "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachines/vm-1",
    "name": "vm-1",
    "vmSize": "Standard_D2s_v3",
    "osType": "Linux",
    "version": "22.04.202401010",
    "subscriptionId": "sub-1",
    "resourceGroupName": "rg-1",
    "vmScaleSetName": "",
    "zone": "1",
    "azEnvironment": "AzurePublicCloud",
    "offer": "0001-com-ubuntu-server-jammy",
    "publisher": "canonical",
    "sku": "22_04-lts-gen2",
    "physicalZone": "westeurope-az1",
    "placementGroupId": "",
    "platformFaultDomain": "0",
    "platformUpdateDomain": "0",
    "priority": "Regular",
    "evictionPolicy": "",
    "licenseType": "",
    "provider": "Microsoft.Compute",
    "osProfile": {
      "computerName": "vm-1-host",
      "adminUsername": "azureuser",
      "disablePasswordAuthentication": "true"
    },
    "publicKeys": [
      { "keyData": "ssh-rsa AAAAB test@azure", "path": "/home/azureuser/.ssh/authorized_keys" }
    ],
    "securityProfile": {
      "secureBootEnabled": "true",
      "virtualTpmEnabled": "true",
      "encryptionAtHost": "false",
      "securityType": "TrustedLaunch"
    },
    "storageProfile": {
      "imageReference": {
        "id": "",
        "offer": "0001-com-ubuntu-server-jammy",
        "publisher": "canonical",
        "sku": "22_04-lts-gen2",
        "version": "22.04.202401010"
      },
      "osDisk": {
        "name": "vm-1_OsDisk_1",
        "caching": "ReadWrite",
        "createOption": "FromImage",
        "diskSizeGB": "30",
        "osType": "Linux",
        "writeAcceleratorEnabled": "false",
        "managedDisk": { "id": "/subscriptions/sub-1/disks/vm-1_OsDisk_1", "storageAccountType": "Premium_LRS" }
      },
      "dataDisks": [
        {
          "name": "vm-1_data_0",
          "caching": "None",
          "createOption": "Attach",
          "diskSizeGB": "128",
          "lun": "0",
          "writeAcceleratorEnabled": "false",
          "managedDisk": { "id": "/subscriptions/sub-1/disks/vm-1_data_0", "storageAccountType": "StandardSSD_LRS" }
        }
      ],
      "resourceDisk": { "size": "16384" }
    },
    "plan": { "name": "", "product": "", "publisher": "" },
    "virtualMachineScaleSet": { "id": "" },
    "tagsList": [
      { "name": "env", "value": "prod" },
      { "name": "team", "value": "platform" }
    ]
  },
  "network": {
    "interface": [
      {
        "macAddress": "000D3AF8E9B2",
        "ipv4": {
          "ipAddress": [
            { "privateIpAddress": "10.0.0.4", "publicIpAddress": "20.50.0.10" }
          ],
          "subnet": [ { "address": "10.0.0.0", "prefix": "24" } ]
        },
        "ipv6": { "ipAddress": [] }
      }
    ]
  }
}`

// The real service answers 400 without the Metadata header, so the fake does
// too: a provider that stopped sending it would otherwise keep passing.
func handler(body string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metadata/instance", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	return mux
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "azure/vm",
		Handler: handler(metadataJSON),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := vm.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler(metadataJSON))

	md, err := vm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, vm.Metadata{
		ID:                "02aab8a4-74ef-476e-8182-f6d2ba4166a6",
		Hostname:          "vm-1-host",
		Name:              "vm-1",
		Region:            "westeurope",
		Zone:              "1",
		ResourceID:        "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachines/vm-1",
		AccountID:         "sub-1",
		Type:              "Standard_D2s_v3",
		OSType:            "Linux",
		OSVersion:         "22.04.202401010",
		ResourceGroupName: "rg-1",
		Tags:              map[string]string{"env": "prod", "team": "platform"},

		Offer:                         "0001-com-ubuntu-server-jammy",
		Publisher:                     "canonical",
		SKU:                           "22_04-lts-gen2",
		AZEnvironment:                 "AzurePublicCloud",
		PhysicalZone:                  "westeurope-az1",
		PlatformFaultDomain:           "0",
		PlatformUpdateDomain:          "0",
		Priority:                      "Regular",
		Provider:                      "Microsoft.Compute",
		AdminUsername:                 "azureuser",
		DisablePasswordAuthentication: "true",
		PublicKeys: []vm.PublicKey{{
			KeyData: "ssh-rsa AAAAB test@azure",
			Path:    "/home/azureuser/.ssh/authorized_keys",
		}},
		SecurityProfile: vm.SecurityProfile{
			SecureBootEnabled: "true", VirtualTpmEnabled: "true",
			EncryptionAtHost: "false", SecurityType: "TrustedLaunch",
		},
		StorageProfile: wantStorage(),
		Interfaces:     wantInterfaces(),
	}, md)
}

// A VM created from a specialised disk reports no computer name.
func TestHostnameFallsBackToVMName(t *testing.T) {
	ep := imdstest.Server(t, handler(`{"compute":{"vmId":"id-1","name":"vm-specialised"}}`))

	md, err := vm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, "vm-specialised", md.Hostname)
}

func TestNoTagsYieldsNoMap(t *testing.T) {
	ep := imdstest.Server(t, handler(`{"compute":{"vmId":"id-1","name":"vm-1"}}`))

	md, err := vm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Nil(t, md.Tags)
}

// Another cloud's metadata service on the shared address serves JSON that
// decodes cleanly into an empty document, since unknown fields are ignored.
func TestDocumentWithoutVMIDIsForeign(t *testing.T) {
	ep := imdstest.Server(t, handler(`{"droplet_id":2756294,"hostname":"sample-droplet"}`))

	_, err := vm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func wantStorage() vm.StorageProfile {
	var sp vm.StorageProfile
	sp.ImageReference = vm.ImageReference{
		Offer: "0001-com-ubuntu-server-jammy", Publisher: "canonical",
		SKU: "22_04-lts-gen2", Version: "22.04.202401010",
	}
	sp.OSDisk = vm.OSDisk{
		Name: "vm-1_OsDisk_1", Caching: "ReadWrite", CreateOption: "FromImage",
		DiskSizeGB: "30", OSType: "Linux", WriteAcceleratorEnabled: "false",
	}
	sp.OSDisk.ManagedDisk.ID = "/subscriptions/sub-1/disks/vm-1_OsDisk_1"
	sp.OSDisk.ManagedDisk.StorageAccountType = "Premium_LRS"

	d := vm.DataDisk{
		Name: "vm-1_data_0", Caching: "None", CreateOption: "Attach",
		DiskSizeGB: "128", Lun: "0", WriteAcceleratorEnabled: "false",
	}
	d.ManagedDisk.ID = "/subscriptions/sub-1/disks/vm-1_data_0"
	d.ManagedDisk.StorageAccountType = "StandardSSD_LRS"
	sp.DataDisks = []vm.DataDisk{d}
	sp.ResourceDisk.Size = "16384"
	return sp
}

func wantInterfaces() []vm.Interface {
	var i vm.Interface
	i.MACAddress = "000D3AF8E9B2"
	i.IPv4.IPAddress = []vm.IPAddress{{PrivateIPAddress: "10.0.0.4", PublicIPAddress: "20.50.0.10"}}
	i.IPv4.Subnet = []vm.Subnet{{Address: "10.0.0.0", Prefix: "24"}}
	i.IPv6.IPAddress = []vm.IPAddress{}
	return []vm.Interface{i}
}
