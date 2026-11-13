import {useTranslator} from '../../lib/i18n'
import * as DialogPrimitive from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import type {ReactNode} from 'react'
export function Dialog({open,onOpenChange,title,description,children,wide=false}:{open:boolean;onOpenChange:(v:boolean)=>void;title:string;description:string;children:ReactNode;wide?:boolean}){const tr=useTranslator();return <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}><DialogPrimitive.Portal><DialogPrimitive.Overlay className="dialog-overlay"/><DialogPrimitive.Content className={`dialog-content ${wide?'dialog-wide':''}`}><div className="dialog-heading"><div><DialogPrimitive.Title>{tr(title)}</DialogPrimitive.Title><DialogPrimitive.Description>{tr(description)}</DialogPrimitive.Description></div><DialogPrimitive.Close className="icon-button" aria-label={tr('关闭')}><X size={19}/></DialogPrimitive.Close></div>{children}</DialogPrimitive.Content></DialogPrimitive.Portal></DialogPrimitive.Root>}
