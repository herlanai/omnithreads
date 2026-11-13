import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFile,writeFile,unlink} from 'node:fs/promises'
import {transpileModule,ModuleKind,JsxEmit,ScriptTarget} from 'typescript'
import {createElement} from 'react'
import {renderToStaticMarkup} from 'react-dom/server'
const temp=new URL('../.markdown-test-'+process.pid+'.mjs',import.meta.url)
const source=await readFile(new URL('../src/components/rich-text.tsx',import.meta.url),'utf8')
await writeFile(temp,transpileModule(source,{compilerOptions:{target:ScriptTarget.ES2022,jsx:JsxEmit.ReactJSX,module:ModuleKind.ESNext}}).outputText)
let RichText
try {({RichText}=await import(temp.href))} finally {await unlink(temp)}
const render=text=>renderToStaticMarkup(createElement(RichText,{text,onMention:()=>{}}))
test('renders model Markdown as headings, emphasis, lists, tables and code',()=>{
 const html=render('# 我的定位\n\n我是**严谨的研究员**。\n\n## 帮助\n\n- 获取证据\n- 交叉核对\n\n|任务|状态|\n|---|---|\n|研究|完成|\n\n```js\nconst x = 1\n```')
 assert.match(html,/<h1>我的定位<\/h1>/);assert.match(html,/<strong>严谨的研究员<\/strong>/);assert.match(html,/<li>获取证据<\/li>/);assert.match(html,/<table>/);assert.match(html,/<pre><code class="language-js">/)
 assert.doesNotMatch(html,/\*\*严谨|# 我的定位/)
})
test('mentions stay interactive but code and link labels remain literal',()=>{
 const html=render('请 **@atlas** 看看，@muse 也参与。\n\n`@byte`\n\n[@echo](https://example.com)\n\n```text\n@atlas\n```')
 assert.equal((html.match(/class="mention"/g)||[]).length,2)
 assert.match(html,/<code>@byte<\/code>/);assert.match(html,/>@echo<\/a>/);assert.match(html,/<code class="language-text">@atlas/)
})
test('does not execute raw HTML or unsafe links',()=>{
 const html=render('<script>alert(1)</script>\n\n[bad](javascript:alert%281%29)\n\n<img src=x onerror=alert(1)>')
 assert.doesNotMatch(html,/<script|<img|javascript:|onerror=/)
})
