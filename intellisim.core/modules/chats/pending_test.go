package chats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPendingTracker(t *testing.T) {
	p := newPendingTracker()

	assert.False(t, p.isThinking("c1"))
	assert.True(t, p.tryStart("c1"))
	assert.True(t, p.isThinking("c1"))
	assert.False(t, p.tryStart("c1"), "second start must be rejected while pending")
	assert.True(t, p.tryStart("c2"), "other chats are independent")

	p.finish("c1")
	assert.False(t, p.isThinking("c1"))
	assert.True(t, p.tryStart("c1"))
}
