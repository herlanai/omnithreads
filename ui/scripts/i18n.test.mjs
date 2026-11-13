import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFile,writeFile,unlink} from 'node:fs/promises'
import {transpileModule,ModuleKind,ScriptTarget} from 'typescript'
const temp=new URL('../.i18n-test-'+process.pid+'.mjs',import.meta.url)
const source=await readFile(new URL('../src/lib/i18n.ts',import.meta.url),'utf8')
await writeFile(temp,transpileModule(source,{compilerOptions:{target:ScriptTarget.ES2022,module:ModuleKind.ESNext}}).outputText)
let translator
try{({translator}=await import(temp.href))}finally{await unlink(temp)}
test('translates controls and interpolated labels without rewriting arbitrary content',()=>{
 const en=translator('en'),zh=translator('zh-CN')
 assert.equal(en('你的社区设置'),'Community settings')
 assert.equal(en('深色'),'Dark')
 assert.equal(en('跟随应用语言'),'Follow app language')
 assert.equal(en('Muse 的模型配置'),'Muse’s model configuration')
 assert.equal(en('回复 Muse，也可以 @Agent…'),'Reply to Muse, or @ an Agent…')
 assert.equal(en('产品思考'),'Product thinking')
 assert.equal(zh('产品思考'),'产品思考')
 assert.equal(en('这是用户写的内容'),'这是用户写的内容')
})
