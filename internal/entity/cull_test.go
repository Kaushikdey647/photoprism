package entity

import (
	"testing"

	"github.com/photoprism/photoprism/pkg/rnd"
)

func TestNewCull(t *testing.T) {
	c := NewCull("ps6sg6be2lvl0yh7", CullSrcAuto)
	if c == nil {
		t.Fatal("expected cull")
	}
	if !rnd.IsUID(c.CullUID, CullUID) {
		t.Fatalf("invalid cull uid %s", c.CullUID)
	}
	if c.KeeperUID != "ps6sg6be2lvl0yh7" {
		t.Fatalf("keeper = %s", c.KeeperUID)
	}
	if c.CullSrc != CullSrcAuto {
		t.Fatalf("src = %s", c.CullSrc)
	}
	if c.TableName() != "culls" {
		t.Fatalf("table = %s", c.TableName())
	}
}

func TestNewPhotoCull(t *testing.T) {
	m := NewPhotoCull("ps6sg6be2lvl0yh7", "ns6sg6be2lvl0yh7", "")
	if m.MemberRole != CullRoleReject {
		t.Fatalf("role = %s, want reject default", m.MemberRole)
	}
	if m.TableName() != "photos_culls" {
		t.Fatalf("table = %s", m.TableName())
	}
}

func TestSrcCull(t *testing.T) {
	if SrcPriority[SrcCull] != 64 {
		t.Fatalf("SrcCull priority = %d", SrcPriority[SrcCull])
	}
	if SrcDesc[SrcCull] == "" {
		t.Fatal("missing SrcCull description")
	}
}
