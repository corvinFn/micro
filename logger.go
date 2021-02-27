/*
 * @Date: 2020-09-27 17:56:51
 * @LastEditors: aiden.deng (Zhenpeng Deng)
 * @LastEditTime: 2020-09-27 18:00:24
 */
package micro

type Logger interface {
	Trace(...interface{})
	Debug(...interface{})
	Info(...interface{})
	Infof(string, ...interface{})
	Warn(...interface{})
	Error(...interface{})
	Errorf(string, ...interface{})
	Panic(...interface{})
}
