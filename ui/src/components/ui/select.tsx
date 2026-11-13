import * as React from 'react'
import * as SelectPrimitive from '@radix-ui/react-select'
import {Check, ChevronDown, ChevronUp} from 'lucide-react'
import {cn} from '../../lib/utils'

// shadcn/ui Select composition, styled with the app's shared theme tokens.
export const Select = SelectPrimitive.Root
export const SelectValue = SelectPrimitive.Value
export function SelectTrigger({className,children,...props}:React.ComponentProps<typeof SelectPrimitive.Trigger>){
 return <SelectPrimitive.Trigger data-slot="select-trigger" className={cn('select-trigger',className)} {...props}>{children}<SelectPrimitive.Icon asChild><ChevronDown size={15} className="select-chevron"/></SelectPrimitive.Icon></SelectPrimitive.Trigger>
}
export function SelectContent({className,children,...props}:React.ComponentProps<typeof SelectPrimitive.Content>){
 return <SelectPrimitive.Portal><SelectPrimitive.Content data-slot="select-content" position="popper" sideOffset={6} collisionPadding={12} className={cn('select-content',className)} {...props}><SelectPrimitive.ScrollUpButton className="select-scroll"><ChevronUp size={14}/></SelectPrimitive.ScrollUpButton><SelectPrimitive.Viewport className="select-viewport">{children}</SelectPrimitive.Viewport><SelectPrimitive.ScrollDownButton className="select-scroll"><ChevronDown size={14}/></SelectPrimitive.ScrollDownButton></SelectPrimitive.Content></SelectPrimitive.Portal>
}
export function SelectItem({className,children,...props}:React.ComponentProps<typeof SelectPrimitive.Item>){
 return <SelectPrimitive.Item data-slot="select-item" className={cn('select-item',className)} {...props}><SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText><span className="select-indicator"><SelectPrimitive.ItemIndicator><Check size={14}/></SelectPrimitive.ItemIndicator></span></SelectPrimitive.Item>
}
export function FieldSelect({children,className,'aria-label':label,...props}:React.ComponentProps<typeof SelectPrimitive.Root>&{'aria-label':string;className?:string}){
 return <Select {...props}><SelectTrigger aria-label={label} className={className}><SelectValue/></SelectTrigger><SelectContent>{children}</SelectContent></Select>
}
