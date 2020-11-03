package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

type DHCP4GlobalParameter struct {
	ID int `gorm:"primary_key"`
}

type DHCP4Params struct {
	BootFileName             *string   `gorm:"column:boot_file_name"`
	NextServer               *int      `gorm:"column:next_server"`
	ServerHostName           *string   `gorm:"column:server_hostname"`
	ClientClass              *string   `gorm:"column:client_class"`
	Interface                *string   `gorm:"column:interface"`
	MatchClientID            bool      `gorm:"column:match_client_id;type:tinyint(1)"`
	Relay                    *string   `gorm:"column:relay;type:longtext"`
	RequireClientClasses     *string   `gorm:"column:require_client_classes;type:longtext"`
	ReservationMode          *int      `gorm:"column:reservation_mode;type:tinyint(3)"`
	Authoritative            bool      `gorm:"column:authoritative;type:tinyint(1)"`
	ValidLifetime            *int      `gorm:"column:valid_lifetime"`
	RebindTimer              *int      `gorm:"column:rebind_timer"`
	RenewTimer               *int      `gorm:"column:renew_timer"`
	CalculateTeeTimes        bool      `gorm:"column:calculate_tee_times;type:tinyint(1)"`
	T1Percent                *float32  `gorm:"column:t1_percent;type:float"`
	T2Percent                *float32  `gorm:"column:t2_percent;type:float"`
	MinValidLifetime         *int      `gorm:"column:min_valid_lifetime"`
	MaxValidLifetime         *int      `gorm:"column:max_valid_lifetime"`
	DDNSSendUpdates          bool      `gorm:"column:ddns_send_updates;type:tinyint(1)"`
	DDNSOverrideNoUpdates    bool      `gorm:"column:ddns_override_no_update;type:tinyint(1)"`
	DDNSOverrideClientUpdate bool      `gorm:"column:ddns_override_client_update;type:tinyint(1)"`
	DDNSReplaceClientName    *int      `gorm:"column:ddns_replace_client_name;type:tinyint(3)"`
	DDNSGeneratedPrefix      *string   `gorm:"column:ddns_generated_prefix"`
	DDNSQualifyingSuffix     *string   `gorm:"column:ddns_qualifying_suffix"`
	UserContext              *string   `gorm:"column:user_context;type:longtext"`
	ModifiedAt               time.Time `gorm:"column:modification_ts;not null;type:timestamp"`
}

// DHCP4SharedNetwork represents DHCP v4 shared network in kea
type DHCP4SharedNetwork struct {
	ID      int            `gorm:"primary key;"`
	Name    string         `gorm:"unique;not null"`
	Servers []*DHCP4Server `gorm:"many2many:dhcp4_shared_network_server;foreignkey:id;jointable_foreignkey:shared_network_id;association_foreignkey:id;association_jointable_foreignkey:server_id;association_autoupdate:false;association_autocreate:false"`
	DHCP4Params
}

// BeforeCreate sets modification timestamp to DHCP4SharedNetwork
func (network *DHCP4SharedNetwork) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("ModifiedAt", time.Now())
	return nil
}

func (network *DHCP4SharedNetwork) Create() error {

}

// DHCP4Subnet represents DHCP v4 subnet in kea
type DHCP4Subnet struct {
	ID                int            `gorm:"primary_key;column:subnet_id"`
	Prefix            string         `gorm:"unique;column:subnet_prefix;not null"`
	V4To6Interface    *string        `gorm:"column:4o6_interface"`
	V4To6InterfaceID  *string        `gorm:"column:4o6_interface_id"`
	V4To6Subnet       *string        `gorm:"column:4o6_subnet"`
	SharedNetworkName *string        `gorm:"column:shared_network_name"`
	Servers           []*DHCP4Server `gorm:"many2many:dhcp4_subnet_server;foreignkey:subnet_id;jointable_foreignkey:subnet_id;association_foreignkey:id;association_jointable_foreignkey:server_id;association_autoupdate:false;association_autocreate:false"`
	DHCP4Params
}

// BeforeCreate sets modification timestamp and id in subnet
func (subnet *DHCP4Subnet) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("ModifiedAt", time.Now())
	// generate id
	lastSubnet4 := DHCP4Subnet{}
	db := scope.DB().Last(&lastSubnet4)
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 1 {
		scope.SetColumn("ID", lastSubnet4.ID+1)
		return nil
	} else if db.RecordNotFound() {
		scope.SetColumn("ID", 1)
		return nil
	}
	return db.Error
}

// DHCP4Server represents DHCP v4 server in kea
type DHCP4Server struct {
	ID          int    `gorm:"primary_key"`
	Tag         string `gorm:"unique;not null"`
	Description string
	ModifiedAt  time.Time `gorm:"column:modification_ts;not null;type:timestamp"`
}

// GetDHCP4Server loads DHCP4Server by tag
func GetDHCP4Server(db *gorm.DB, tag string) *DHCP4Server {
	dhcp4Server := &DHCP4Server{}
	result := db.Table("dhcp4_server").Where("tag=?", tag).Scan(dhcp4Server)
	if result.RowsAffected == 0 || dhcp4Server.Tag == "" {
		return nil
	}
	return dhcp4Server
}

// GetSharedNetwork4 loads DHCP4SharedNetwork by name
func GetSharedNetwork4(db *gorm.DB, name string) *DHCP4SharedNetwork {
	sharedNetwork := &DHCP4SharedNetwork{}
	result := db.Table("dhcp4_shared_network").Where("name=?", name).Scan(sharedNetwork)
	if result.RowsAffected == 0 || sharedNetwork.Name == "" {
		return nil
	}
	return sharedNetwork
}

func audit(db *gorm.DB, serverTag, auditMessage string) *gorm.DB {
	tx := db.Begin()
	tx.Exec("call createAuditRevisionDHCP4(?,?,?,?)", time.Now(), serverTag, auditMessage, true)
	return tx
}

func CreateSharedNetwork4(db *gorm.DB, server *DHCP4Server, sharedNetwork *DHCP4SharedNetwork) error {
	tx := db.Begin()
	auditMsg := fmt.Sprintf("add new shared network: %s", sharedNetwork.Name)
	tx.Exec("call createAuditRevisionDHCP4(?,?,?,?)", time.Now(), server.Tag, auditMsg, true)
	servers := sharedNetwork.Servers
	if servers == nil {
		servers = make([]*DHCP4Server, 0)
	}
	servers = append(servers, server)
	sharedNetwork.Servers = servers
	if result := tx.Table("dhcp4_shared_network").Create(sharedNetwork); result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	return tx.Commit().Error
}

func CreateSubnet4(db *gorm.DB, server *DHCP4Server, sharedNetwork *DHCP4SharedNetwork, subnet *DHCP4Subnet) error {
	audit(db, server.Tag)
	tx := db.Begin()
	auditMsg := fmt.Sprintf("add new subnet: %s", subnet.Prefix)
	tx.Exec("call createAuditRevisionDHCP4(?,?,?,?)", time.Now(), server.Tag, auditMsg, true)
	subnet.SharedNetworkName = &sharedNetwork.Name
	servers := subnet.Servers
	if servers == nil {
		servers = make([]*DHCP4Server, 0)
	}
	servers = append(servers, server)
	subnet.Servers = servers
	if ret := tx.Table("dhcp4_subnet").Create(subnet); ret.Error != nil {
		tx.Rollback()
		return ret.Error
	}
	return tx.Commit().Error
}

func main() {
	db, err := gorm.Open("mysql", "kea:kea@(192.168.1.12)/kea?parseTime=true")
	if err != nil {
		panic("failed to connect database")
	}
	db.LogMode(true)
	defer db.Close()

	dhcp4Server := GetDHCP4Server(db, "all")
	if dhcp4Server == nil {
		panic("'all' server not defined")
	}
	sharedNetworkName := "network_test1"
	/*sharedNetwork := &DHCP4SharedNetwork{
		Name: sharedNetworkName,
	}
	if err := CreateSharedNetwork4(db, dhcp4Server, sharedNetwork); err != nil {
		fmt.Printf("failed to create shared network %s: %s", sharedNetwork.Name, err.Error())
		return
	}*/
	sharedNetwork := GetSharedNetwork4(db, sharedNetworkName)
	if sharedNetwork == nil {
		fmt.Printf("shared network notfound %s: %s", sharedNetworkName, err.Error())
		os.Exit(1)
	}
	subnet := &DHCP4Subnet{
		Prefix: "192.168.101.0/24",
	}
	if err := CreateSubnet4(db, dhcp4Server, sharedNetwork, subnet); err != nil {
		fmt.Printf("failed to create subnet %s: %s", subnet.Prefix, err.Error())
	}
}
