import { DropdownMenu as DropdownMenuPrimitive } from 'bits-ui';
import Content from './dropdown-menu-content.svelte';
import Item from './dropdown-menu-item.svelte';
import Separator from './dropdown-menu-separator.svelte';

const Root = DropdownMenuPrimitive.Root;
const Trigger = DropdownMenuPrimitive.Trigger;
const RadioGroup = DropdownMenuPrimitive.RadioGroup;
const RadioItem = DropdownMenuPrimitive.RadioItem;

export {
	Root,
	Trigger,
	Content,
	Item,
	Separator,
	RadioGroup,
	RadioItem,
	//
	Root as DropdownMenu,
	Trigger as DropdownMenuTrigger,
	Content as DropdownMenuContent,
	Item as DropdownMenuItem,
	Separator as DropdownMenuSeparator,
	RadioGroup as DropdownMenuRadioGroup,
	RadioItem as DropdownMenuRadioItem
};
