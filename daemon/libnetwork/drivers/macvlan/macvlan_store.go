//go:build linux

package macvlan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const (
	macvlanPrefix         = "macvlan"
	macvlanNetworkPrefix  = macvlanPrefix + "/network"
	macvlanEndpointPrefix = macvlanPrefix + "/endpoint"
)

// networkConfiguration for this driver's network specific configuration
type configuration struct {
	ID               string
	Mtu              int
	dbIndex          uint64
	dbExists         bool
	Internal         bool
	Parent           string
	MacvlanMode      string
	CreatedSlaveLink bool
	Ipv4Subnets      []*ipSubnet
	Ipv6Subnets      []*ipSubnet
}

type ipSubnet struct {
	SubnetIP string
	GwIP     string
}

// initStore drivers are responsible for caching their own persistent state
func (d *driver) initStore() error {
	err := d.populateNetworks()
	if err != nil {
		return err
	}
	err = d.populateEndpoints()
	if err != nil {
		return err
	}

	return nil
}

// populateNetworks is invoked at driver init to recreate persistently stored networks
func (d *driver) populateNetworks() error {
	kvol, err := d.store.List(&configuration{})
	if err != nil && !errors.Is(err, datastore.ErrKeyNotFound) {
		return fmt.Errorf("failed to get macvlan network configurations from store: %w", err)
	}
	// If empty it simply means no macvlan networks have been created yet
	if errors.Is(err, datastore.ErrKeyNotFound) {
		return nil
	}
	for _, kvo := range kvol {
		config := kvo.(*configuration)
		if _, err = d.createNetwork(config); err != nil {
			log.G(context.TODO()).Warnf("Could not create macvlan network for id %s from persistent state", config.ID)
		}
	}

	return nil
}

func (d *driver) populateEndpoints() error {
	kvol, err := d.store.List(&endpoint{})
	if err != nil && !errors.Is(err, datastore.ErrKeyNotFound) {
		return fmt.Errorf("failed to get macvlan endpoints from store: %w", err)
	}

	if errors.Is(err, datastore.ErrKeyNotFound) {
		return nil
	}

	for _, kvo := range kvol {
		ep := kvo.(*endpoint)
		n, ok := d.networks[ep.nid]
		if !ok {
			log.G(context.TODO()).Debugf("Network (%.7s) not found for restored macvlan endpoint (%.7s)", ep.nid, ep.id)
			log.G(context.TODO()).Debugf("Deleting stale macvlan endpoint (%.7s) from store", ep.id)
			if err := d.storeDelete(ep); err != nil {
				log.G(context.TODO()).Debugf("Failed to delete stale macvlan endpoint (%.7s) from store", ep.id)
			}
			continue
		}
		n.endpoints[ep.id] = ep
		log.G(context.TODO()).Debugf("Endpoint (%.7s) restored to network (%.7s)", ep.id, ep.nid)
	}

	return nil
}

// storeUpdate used to update persistent macvlan network records as they are created
func (d *driver) storeUpdate(kvObject datastore.KVObject) error {
	if d.store == nil {
		log.G(context.TODO()).Warnf("macvlan store not initialized. kv object %s is not added to the store", datastore.Key(kvObject.Key()...))
		return nil
	}
	if err := d.store.PutObjectAtomic(kvObject); err != nil {
		return fmt.Errorf("failed to update macvlan store for object type %T: %v", kvObject, err)
	}

	return nil
}

// storeDelete used to delete macvlan records from persistent cache as they are deleted
func (d *driver) storeDelete(kvObject datastore.KVObject) error {
	if d.store == nil {
		log.G(context.TODO()).Debugf("macvlan store not initialized. kv object %s is not deleted from store", datastore.Key(kvObject.Key()...))
		return nil
	}

	return d.store.DeleteObject(kvObject)
}

func (config *configuration) MarshalJSON() ([]byte, error) {
	nMap := make(map[string]interface{})
	nMap["ID"] = config.ID
	nMap["Mtu"] = config.Mtu
	nMap["Parent"] = config.Parent
	nMap["MacvlanMode"] = config.MacvlanMode
	nMap["Internal"] = config.Internal
	nMap["CreatedSubIface"] = config.CreatedSlaveLink
	if len(config.Ipv4Subnets) > 0 {
		iis, err := json.Marshal(config.Ipv4Subnets)
		if err != nil {
			return nil, err
		}
		nMap["Ipv4Subnets"] = string(iis)
	}
	if len(config.Ipv6Subnets) > 0 {
		iis, err := json.Marshal(config.Ipv6Subnets)
		if err != nil {
			return nil, err
		}
		nMap["Ipv6Subnets"] = string(iis)
	}

	return json.Marshal(nMap)
}

func (config *configuration) UnmarshalJSON(b []byte) error {
	var (
		err  error
		nMap map[string]interface{}
	)

	if err = json.Unmarshal(b, &nMap); err != nil {
		return err
	}
	config.ID = nMap["ID"].(string)
	config.Mtu = int(nMap["Mtu"].(float64))
	config.Parent = nMap["Parent"].(string)
	config.MacvlanMode = nMap["MacvlanMode"].(string)
	config.Internal = nMap["Internal"].(bool)
	config.CreatedSlaveLink = nMap["CreatedSubIface"].(bool)
	if v, ok := nMap["Ipv4Subnets"]; ok {
		if err := json.Unmarshal([]byte(v.(string)), &config.Ipv4Subnets); err != nil {
			return err
		}
	}
	if v, ok := nMap["Ipv6Subnets"]; ok {
		if err := json.Unmarshal([]byte(v.(string)), &config.Ipv6Subnets); err != nil {
			return err
		}
	}

	return nil
}

func (config *configuration) Key() []string {
	return []string{macvlanNetworkPrefix, config.ID}
}

func (config *configuration) KeyPrefix() []string {
	return []string{macvlanNetworkPrefix}
}

func (config *configuration) Value() []byte {
	b, err := json.Marshal(config)
	if err != nil {
		return nil
	}

	return b
}

func (config *configuration) SetValue(value []byte) error {
	return json.Unmarshal(value, config)
}

func (config *configuration) Index() uint64 {
	return config.dbIndex
}

func (config *configuration) SetIndex(index uint64) {
	config.dbIndex = index
	config.dbExists = true
}

func (config *configuration) Exists() bool {
	return config.dbExists
}

func (config *configuration) Skip() bool {
	return false
}

func (config *configuration) New() datastore.KVObject {
	return &configuration{}
}

func (config *configuration) CopyTo(o datastore.KVObject) error {
	dstNcfg := o.(*configuration)
	*dstNcfg = *config

	return nil
}

func (ep *endpoint) MarshalJSON() ([]byte, error) {
	epMap := make(map[string]interface{})
	epMap["id"] = ep.id
	epMap["nid"] = ep.nid
	epMap["SrcName"] = ep.srcName
	if len(ep.mac) != 0 {
		epMap["MacAddress"] = ep.mac.String()
	}
	if ep.addr != nil {
		epMap["Addr"] = ep.addr.String()
	}
	if ep.addrv6 != nil {
		epMap["Addrv6"] = ep.addrv6.String()
	}
	return json.Marshal(epMap)
}

func (ep *endpoint) UnmarshalJSON(b []byte) error {
	var (
		err   error
		epMap map[string]interface{}
	)

	if err = json.Unmarshal(b, &epMap); err != nil {
		return fmt.Errorf("Failed to unmarshal to macvlan endpoint: %v", err)
	}

	if v, ok := epMap["MacAddress"]; ok {
		if ep.mac, err = net.ParseMAC(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode macvlan endpoint MAC address (%s) after json unmarshal: %v", v.(string), err)
		}
	}
	if v, ok := epMap["Addr"]; ok {
		if ep.addr, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode macvlan endpoint IPv4 address (%s) after json unmarshal: %v", v.(string), err)
		}
	}
	if v, ok := epMap["Addrv6"]; ok {
		if ep.addrv6, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode macvlan endpoint IPv6 address (%s) after json unmarshal: %v", v.(string), err)
		}
	}
	ep.id = epMap["id"].(string)
	ep.nid = epMap["nid"].(string)
	ep.srcName = epMap["SrcName"].(string)

	return nil
}

func (ep *endpoint) Key() []string {
	return []string{macvlanEndpointPrefix, ep.id}
}

func (ep *endpoint) KeyPrefix() []string {
	return []string{macvlanEndpointPrefix}
}

func (ep *endpoint) Value() []byte {
	b, err := json.Marshal(ep)
	if err != nil {
		return nil
	}
	return b
}

func (ep *endpoint) SetValue(value []byte) error {
	return json.Unmarshal(value, ep)
}

func (ep *endpoint) Index() uint64 {
	return ep.dbIndex
}

func (ep *endpoint) SetIndex(index uint64) {
	ep.dbIndex = index
	ep.dbExists = true
}

func (ep *endpoint) Exists() bool {
	return ep.dbExists
}

func (ep *endpoint) Skip() bool {
	return false
}

func (ep *endpoint) New() datastore.KVObject {
	return &endpoint{}
}

func (ep *endpoint) CopyTo(o datastore.KVObject) error {
	dstEp := o.(*endpoint)
	*dstEp = *ep
	return nil
}
