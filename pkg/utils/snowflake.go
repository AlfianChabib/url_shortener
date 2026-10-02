package utils

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

// NewSnowflakeNode initializes a new Snowflake node with the specified node ID.
func NewSnowflakeNode(nodeID int64) (*snowflake.Node, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to create snowflake node with id %d: %w", nodeID, err)
	}
	return node, nil
}

