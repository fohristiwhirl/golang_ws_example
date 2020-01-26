package app

var conn_id_generator = ConnIdGenerator{}

type ConnIdGenerator struct {
	val				int
}

func (self *ConnIdGenerator) Next() int {
	self.val += 1
	return self.val
}
