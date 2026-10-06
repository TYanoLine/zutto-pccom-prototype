package worldcatalog

import "testing"

func TestMakeCenterIsDeterministic(t *testing.T) {
	a := makeCenter("TEST BBS", 123456789, 12, 0)
	b := makeCenter("TEST BBS", 123456789, 12, 0)
	if a != b { t.Fatalf("same seed/index/generation produced different centers: %#v != %#v", a, b) }
	if a.ID != "world-013" { t.Fatalf("unexpected directory id: %s", a.ID) }
	if a.SoftwareFamily == "" || a.FoundedOn == "" || a.LineCount < 1 || a.MemberCount < 1 { t.Fatalf("incomplete skeleton: %#v", a) }
}

func TestMakeCenterGenerationChangesSkeletonButKeepsDirectoryIdentity(t *testing.T) {
	a := makeCenter("OLD", 42, 4, 0)
	b := makeCenter("NEW", 42, 4, 1)
	if a.ID != b.ID { t.Fatalf("reset changed directory identity: %s != %s", a.ID, b.ID) }
	if a.Phone != b.Phone { t.Fatalf("reset changed directory phone: %s != %s", a.Phone, b.Phone) }
	if a == b { t.Fatal("generation increment did not change host") }
}

func TestWorldKeyValidation(t *testing.T) {
	if !ValidWorldKey("0123456789abcdef0123456789abcdef") { t.Fatal("valid world key rejected") }
	for _, key := range []string{"", "abc", "0123456789ABCDEF0123456789ABCDEF", "0123456789abcdef0123456789abcdeg"} {
		if ValidWorldKey(key) { t.Fatalf("invalid world key accepted: %q", key) }
	}
}

func TestMakeCenterMaintainsPhoneAcrossGenerations(t *testing.T) {
	seed := int64(987654321)
	for index := 0; index < 10; index++ {
		c0 := makeCenter("GEN0", seed, index, 0)
		c1 := makeCenter("GEN1", seed, index, 1)
		c2 := makeCenter("GEN2", seed, index, 2)
		if c0.Phone != c1.Phone || c1.Phone != c2.Phone {
			t.Fatalf("index %d phone changed across generations: %s, %s, %s", index, c0.Phone, c1.Phone, c2.Phone)
		}
		if c0.ID != c1.ID || c1.ID != c2.ID {
			t.Fatalf("index %d id changed across generations: %s, %s, %s", index, c0.ID, c1.ID, c2.ID)
		}
		if c0.Name == c1.Name || c1.Name == c2.Name {
			t.Fatalf("index %d name unexpectedly identical: %s", index, c0.Name)
		}
	}
}

