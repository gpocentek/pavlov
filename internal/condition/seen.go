package condition

type SeenCondition struct {
}

func (c *SeenCondition) String() string {
	return "seen()"
}

func (c *SeenCondition) Eval(ctx *ConditionContext) bool {
	return true
}

func (c *SeenCondition) Validate() error {
	return nil
}
