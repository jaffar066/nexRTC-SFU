package rtp

// TimestampStep calculates how many RTP timestamp ticks
// should pass between media frames.
//
// clockRate = RTP clock rate.
// fps       = frames per second.
func TimestampStep(clockRate uint32, fps uint32) uint32 {
	if fps == 0 {
		return 0
	}

	return clockRate / fps
}

type TimestampGenerator struct {
	current uint32
	step    uint32
}

func NewTimestampGenerator(
	start uint32,
	clockRate uint32,
	fps uint32,
) *TimestampGenerator {

	return &TimestampGenerator{
		current: start,
		step:    TimestampStep(clockRate, fps),
	}
}

func (g *TimestampGenerator) Next() uint32 {
	timestamp := g.current

	g.current += g.step

	return timestamp
}

