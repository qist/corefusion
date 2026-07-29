import * as mc from '@intlify/message-compiler'
import zh from './src/locales/zh-CN.js'
import en from './src/locales/en.js'

const compile = mc.compile || mc.baseCompile
if (!compile) {
  console.error('no compile export, keys=', Object.keys(mc))
  process.exit(2)
}

let fail = 0
function walk(o, path) {
  if (typeof o === 'string') {
    try {
      compile(o)
    } catch (e) {
      console.error('FAIL', path, '->', e.message)
      fail++
    }
  } else if (o && typeof o === 'object') {
    for (const [k, v] of Object.entries(o)) walk(v, path ? path + '.' + k : k)
  }
}
walk(zh, 'zh')
walk(en, 'en')
console.log(fail === 0 ? 'ALL MESSAGES OK' : `FAILED ${fail}`)
process.exit(fail === 0 ? 0 : 1)
