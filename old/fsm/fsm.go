package fsm

type Stater interface {
	Enter()
	Exit()
	CheckTransition(event int) bool
}

type FSM struct {
	// 持有状态集合
	states map[string]Stater
	// 当前状态
	currentState Stater
	// 默认状态
	defaultState Stater
	// 外部输入数据
	inputData int
	// 是否初始化
	inited bool
}

// 重置
func (this *FSM) Reset() {
	this.inited = false
}

// 初始化FSM
func (this *FSM) Init() {
	this.Reset()
}

// 添加状态到FSM
func (this *FSM) AddState(key string, state Stater) {
	if this.states == nil {
		this.states = make(map[string]Stater, 2)
	}
	this.states[key] = state
}

// 设置默认的State
func (this *FSM) SetDefaultState(state Stater) {
	this.defaultState = state
}

// 转移状态
func (this *FSM) TransitionState() {
	inputData := this.inputData
	/*
		nextState := this.defaultState
		if this.inited {
			for _, v := range this.states {
				if v.Can(inputData) {
					nextState = v
					break
				}
			}
		}
	*/
	if !this.inited {
		this.currentState = this.defaultState
	}

	for _, next := range this.states {
		if next.CheckTransition(inputData) {
			if this.currentState != nil {
				this.currentState.Exit()
			}
			this.currentState = next
			this.inited = true
			next.Enter()
			break
		}
	}

	/*
		if ok := nextState.CheckTransition(inputData); ok {
			if this.currentState != nil {
				// 退出前一个状态
				this.currentState.Exit()
			}
			this.currentState = nextState
			this.inited = true
			nextState.Enter()
		}
	*/
}

// 输入数据
func (this *FSM) InputData(inputData int) {
	this.inputData = inputData
	this.TransitionState()
}
