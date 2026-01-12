package vrclog_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

func TestDefaultParser_StandardLog(t *testing.T) {
	p := vrclog.DefaultParser{}
	ctx := context.Background()

	tests := []struct {
		name      string
		line      string
		wantMatch bool
		wantType  event.Type
	}{
		{
			name:      "player_join",
			line:      "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
			wantMatch: true,
			wantType:  event.PlayerJoin,
		},
		{
			name:      "player_left",
			line:      "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser",
			wantMatch: true,
			wantType:  event.PlayerLeft,
		},
		{
			name:      "world_join",
			line:      "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World",
			wantMatch: true,
			wantType:  event.WorldJoin,
		},
		{
			name:      "unrecognized",
			line:      "random text",
			wantMatch: false,
		},
		{
			name:      "empty",
			line:      "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.ParseLine(ctx, tt.line)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMatch, result.Matched)
			if tt.wantMatch {
				require.Len(t, result.Events, 1)
				assert.Equal(t, tt.wantType, result.Events[0].Type)
			} else {
				assert.Empty(t, result.Events)
			}
		})
	}
}

func TestParserFunc(t *testing.T) {
	called := false
	p := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		called = true
		assert.Equal(t, "test line", line)
		return vrclog.ParseResult{Matched: true}, nil
	})

	result, err := p.ParseLine(context.Background(), "test line")
	require.NoError(t, err)
	assert.True(t, called)
	assert.True(t, result.Matched)
}

func TestParserChain_ChainAll(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type1"}},
			Matched: true,
		}, nil
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.Len(t, result.Events, 2)
	assert.Equal(t, event.Type("type1"), result.Events[0].Type)
	assert.Equal(t, event.Type("type2"), result.Events[1].Type)
}

func TestParserChain_ChainFirst(t *testing.T) {
	callOrder := []int{}
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 1)
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type1"}},
			Matched: true,
		}, nil
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 2)
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainFirst,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.Len(t, result.Events, 1)
	assert.Equal(t, []int{1}, callOrder) // p2 should not be called
}

func TestParserChain_ChainFirst_NoMatch(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{Matched: false}, nil
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainFirst,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.Len(t, result.Events, 1)
	assert.Equal(t, event.Type("type2"), result.Events[0].Type)
}

func TestParserChain_ChainContinueOnError(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{}, errors.New("p1 error")
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainContinueOnError,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	assert.Error(t, err) // Error should be returned
	assert.Contains(t, err.Error(), "p1 error")
	assert.True(t, result.Matched) // p2's result should be included
	assert.Len(t, result.Events, 1)
}

func TestParserChain_ChainContinueOnError_AllErrors(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{}, errors.New("p1 error")
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{}, errors.New("p2 error")
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainContinueOnError,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "p1 error")
	assert.Contains(t, err.Error(), "p2 error")
	assert.False(t, result.Matched)
	assert.Empty(t, result.Events)
}

func TestParserChain_NoMatch(t *testing.T) {
	p := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{Matched: false}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{p},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.False(t, result.Matched)
	assert.Empty(t, result.Events)
}

func TestParserChain_Empty(t *testing.T) {
	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.False(t, result.Matched)
	assert.Empty(t, result.Events)
}

func TestParserChain_ErrorStopsChainAll(t *testing.T) {
	callOrder := []int{}
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 1)
		return vrclog.ParseResult{}, errors.New("error")
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 2)
		return vrclog.ParseResult{Matched: true}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{p1, p2},
	}

	_, err := chain.ParseLine(context.Background(), "test")
	assert.Error(t, err)
	assert.Equal(t, []int{1}, callOrder) // p2 should not be called
}

func TestParserChain_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	callCount := 0
	slowParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callCount++
		return vrclog.ParseResult{Matched: false}, nil
	})

	// Create a chain with 10 parsers
	parsers := make([]vrclog.Parser, 10)
	for i := range parsers {
		parsers[i] = slowParser
	}

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: parsers,
	}

	result, err := chain.ParseLine(ctx, "test")
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.False(t, result.Matched)
	// Should stop after checking context, not call all parsers
	assert.Equal(t, 0, callCount, "no parsers should be called when context is already cancelled")
}

func TestParserChain_ContextCancellationMidChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var callCount int32
	parser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		// Use atomic to avoid race detector warnings and demonstrate best practice
		count := atomic.AddInt32(&callCount, 1)
		if count == 5 {
			cancel() // Cancel after 5th call
		}
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "test"}},
			Matched: true,
		}, nil
	})

	parsers := make([]vrclog.Parser, 10)
	for i := range parsers {
		parsers[i] = parser
	}

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: parsers,
	}

	result, err := chain.ParseLine(ctx, "test")
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.True(t, result.Matched)
	// Should have called 5 parsers before cancellation
	assert.Equal(t, int32(5), atomic.LoadInt32(&callCount))
	// Should have collected 5 events before cancellation
	assert.Len(t, result.Events, 5)
}

func TestParserChain_NilParser(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type1"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{p1, nil, p1}, // nil in the middle
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
	// Should skip nil parser and call p1 twice
	assert.Len(t, result.Events, 2)
}

func TestParserChain_ChainContinueOnError_ContextCancellationPreservesErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var callCount int32
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		atomic.AddInt32(&callCount, 1)
		return vrclog.ParseResult{}, errors.New("p1 error")
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		atomic.AddInt32(&callCount, 1)
		// Cancel context during p2 execution
		cancel()
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "test"}},
			Matched: true,
		}, nil
	})
	p3 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		// This should not be called - context cancelled before reaching here
		atomic.AddInt32(&callCount, 1)
		return vrclog.ParseResult{}, errors.New("p3 error")
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainContinueOnError,
		Parsers: []vrclog.Parser{p1, p2, p3},
	}

	result, err := chain.ParseLine(ctx, "test")

	// Should have error (both p1 error and context.Canceled)
	assert.Error(t, err)

	// Error should contain both p1 error and context error
	assert.Contains(t, err.Error(), "p1 error")
	assert.ErrorIs(t, err, context.Canceled)

	// Should have collected event from p2
	assert.True(t, result.Matched)
	assert.Len(t, result.Events, 1)

	// Should have called p1 and p2, but not p3 (cancelled after p2)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}
