package worldengine

type PostKind string

const (
	PostKindNewPost PostKind = "new_post"
	PostKindReply   PostKind = "reply"
)

type BodyLengthBand struct {
	Min    int
	Max    int
	Weight int
}

var replyBodyLengthBands = []BodyLengthBand{
	{1, 40, 15}, {41, 80, 67}, {81, 160, 152}, {161, 320, 152},
	{321, 640, 58}, {641, 1280, 20}, {1281, 2560, 1},
}

var newPostBodyLengthBands = []BodyLengthBand{
	{1, 40, 2}, {41, 80, 13}, {81, 160, 23}, {161, 320, 52},
	{321, 640, 27}, {641, 1280, 15}, {1281, 2560, 8}, {2561, 8192, 8},
}

func SelectBodyLengthBand(seed uint64, kind PostKind) BodyLengthBand {
	bands := replyBodyLengthBands
	if kind != PostKindReply {
		bands = newPostBodyLengthBands
	}
	total := 0
	for _, band := range bands {
		total += band.Weight
	}
	pick := int(seed % uint64(total))
	for _, band := range bands {
		if pick < band.Weight {
			return band
		}
		pick -= band.Weight
	}
	return bands[len(bands)-1]
}
