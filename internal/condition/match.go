package condition

type MatchCondition struct {
}

func (c *MatchCondition) String() string {
	return "match()"
}

func (c *MatchCondition) Eval(ctx *ConditionContext) bool {
	return true
}

func (c *MatchCondition) Validate() error {
	return nil
}
