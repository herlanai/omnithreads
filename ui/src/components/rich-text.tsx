import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkBreaks from 'remark-breaks'
import type {Root, Element, Text} from 'hast'

// Match mentions only in ordinary text: never rewrite code, URLs or link labels.
function mentionNodes() {
  return (tree: Root) => {
    function walk(parent: Root | Element) {
      if (parent.type === 'element' && ['a', 'code', 'pre'].includes(parent.tagName)) return
      for (let i = parent.children.length - 1; i >= 0; i--) {
        const child = parent.children[i]
        if (child.type === 'element') { walk(child); continue }
        if (child.type !== 'text') continue
        const parts: (Text | Element)[] = []
        const pattern = /(^|[^\w@])@([a-zA-Z][a-zA-Z0-9_]{1,23})\b/g
        let offset = 0
        for (const match of child.value.matchAll(pattern)) {
          const start = match.index! + match[1].length
          if (start > offset) parts.push({type: 'text', value: child.value.slice(offset, start)})
          parts.push({type: 'element', tagName: 'span', properties: {dataMention: match[2]}, children: [{type: 'text', value: '@' + match[2]}]})
          offset = start + match[2].length + 1
        }
        if (offset < child.value.length) parts.push({type: 'text', value: child.value.slice(offset)})
        parent.children.splice(i, 1, ...parts)
      }
    }
    walk(tree)
  }
}

export function RichText({text, onMention}: {text: string; onMention?: (handle: string) => void}) {
  return <div className="post-text markdown-body">
    <Markdown remarkPlugins={[remarkGfm, remarkBreaks]} rehypePlugins={[mentionNodes]} skipHtml components={{
      span: ({node, children}) => {
        const handle = node?.properties.dataMention
        return typeof handle === 'string' && onMention
          ? <button type="button" className="mention" onClick={() => onMention(handle)}>{children}</button>
          : <span>{children}</span>
      },
      a: ({children, href, title}) => <a href={href} title={title} target="_blank" rel="noopener noreferrer">{children}</a>,
      table: ({children}) => <div className="markdown-table"><table>{children}</table></div>,
    }}>{text}</Markdown>
  </div>
}
