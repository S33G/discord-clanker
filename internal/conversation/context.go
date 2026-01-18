package conversation

// ContextBuilder builds context for LLM requests with token management
type ContextBuilder struct {
	counter       *TokenCounter
	maxTokens     int
	reserveTokens int // Reserve for response
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(maxTokens, reserveTokens int) *ContextBuilder {
	return &ContextBuilder{
		counter:       NewTokenCounter(),
		maxTokens:     maxTokens,
		reserveTokens: reserveTokens,
	}
}

// Build creates a context from messages, truncating if necessary
// Returns messages that fit within the token budget
func (cb *ContextBuilder) Build(messages []Message, systemPrompt, model string) ([]Message, int, error) {
	availableTokens := cb.maxTokens - cb.reserveTokens

	// System prompt always included
	systemTokens, err := cb.counter.Count(systemPrompt, model)
	if err != nil {
		return nil, 0, err
	}
	systemTokens += 4 // Message formatting overhead

	remainingTokens := availableTokens - systemTokens
	if remainingTokens <= 0 {
		// System prompt alone exceeds budget
		return []Message{
			{Role: "system", Content: systemPrompt, Tokens: systemTokens},
		}, systemTokens, nil
	}

	// Build result from newest to oldest, then reverse
	result := []Message{
		{Role: "system", Content: systemPrompt, Tokens: systemTokens},
	}
	totalTokens := systemTokens

	// Process messages in reverse (keep newest)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// Count tokens for this message
		msgTokens := msg.Tokens
		if msgTokens == 0 {
			// Count if not pre-counted
			count, err := cb.counter.Count(msg.Content, model)
			if err != nil {
				return nil, 0, err
			}
			msgTokens = count + 4 // Message formatting
		}

		// Check if message fits
		if totalTokens+msgTokens > availableTokens {
			// Can't fit more messages
			break
		}

		// Add message (will reverse later)
		result = append(result, msg)
		totalTokens += msgTokens
	}

	// Reverse messages (except system prompt at index 0)
	if len(result) > 1 {
		for i, j := 1, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}

	return result, totalTokens, nil
}

// CountTokens counts tokens for a message
func (cb *ContextBuilder) CountTokens(content, model string) (int, error) {
	return cb.counter.Count(content, model)
}

// WillFit checks if a new message will fit in the current context
func (cb *ContextBuilder) WillFit(currentTokens, newMessageTokens int) bool {
	return currentTokens+newMessageTokens+cb.reserveTokens <= cb.maxTokens
}
