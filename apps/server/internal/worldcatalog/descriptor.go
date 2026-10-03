package worldcatalog

import "zutto-pccom/apps/server/internal/hostcatalog"

// Descriptor converts a generated Center into the canonical HostDescriptor.
//
// Generated centers are currently all listed in the directory (the hosts.listed
// column does not exist yet) and have no region, traits or generation of their
// own; traits use hostcatalog.DefaultTraits until the skeleton generates them.
func (c Center) Descriptor() hostcatalog.HostDescriptor {
	return hostcatalog.HostDescriptor{
		Key:        c.ID,
		Origin:     hostcatalog.OriginGenerated,
		Listed:     true,
		Phone:      c.Phone,
		Name:       c.Name,
		Program:    c.SoftwareFamily,
		Lines:      c.LineCount,
		MaxBaud:    c.MaxBaud,
		FoundedOn:  c.FoundedOn,
		Popularity: c.Popularity,
		Members:    c.MemberCount,
		Traits:     hostcatalog.DefaultTraits(),
		DialMode:   c.DialMode,
		Dial:       hostcatalog.DialBehavior{Kind: hostcatalog.DialNormal},
	}
}

// CenterFromDescriptor is the inverse of Center.Descriptor for the fields a
// Center carries.
func CenterFromDescriptor(d hostcatalog.HostDescriptor) Center {
	return Center{
		ID:             d.Key,
		Name:           d.Name,
		Phone:          d.Phone,
		DialMode:       d.DialMode,
		MaxBaud:        d.MaxBaud,
		SoftwareFamily: d.Program,
		LineCount:      d.Lines,
		FoundedOn:      d.FoundedOn,
		Popularity:     d.Popularity,
		MemberCount:    d.Members,
	}
}
