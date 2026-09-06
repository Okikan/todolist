export namespace main {
	
	export class ImportResult {
	    imported: number;
	    skipped: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.skipped = source["skipped"];
	    }
	}
	export class Settings {
	    dark_mode: boolean;
	    new_task_hotkey: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dark_mode = source["dark_mode"];
	        this.new_task_hotkey = source["new_task_hotkey"];
	    }
	}
	export class TagDef {
	    name: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new TagDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	    }
	}
	export class Todo {
	    id: number;
	    title: string;
	    done: boolean;
	    position: number;
	    created_at: number;
	    completed_at?: number;
	    due_at?: number;
	    urge_days: number;
	    note: string;
	    tag: string;
	    urge: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Todo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.done = source["done"];
	        this.position = source["position"];
	        this.created_at = source["created_at"];
	        this.completed_at = source["completed_at"];
	        this.due_at = source["due_at"];
	        this.urge_days = source["urge_days"];
	        this.note = source["note"];
	        this.tag = source["tag"];
	        this.urge = source["urge"];
	    }
	}
	export class TrashItem {
	    id: number;
	    title: string;
	    done: boolean;
	    created_at: number;
	    completed_at?: number;
	    due_at?: number;
	    urge_days: number;
	    note: string;
	    tag: string;
	    deleted_at: number;
	
	    static createFrom(source: any = {}) {
	        return new TrashItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.done = source["done"];
	        this.created_at = source["created_at"];
	        this.completed_at = source["completed_at"];
	        this.due_at = source["due_at"];
	        this.urge_days = source["urge_days"];
	        this.note = source["note"];
	        this.tag = source["tag"];
	        this.deleted_at = source["deleted_at"];
	    }
	}

}

