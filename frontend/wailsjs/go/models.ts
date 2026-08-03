export namespace armordata {
	
	export class Part {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Part(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Slot {
	    name: string;
	    parts: Part[];
	
	    static createFrom(source: any = {}) {
	        return new Slot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.parts = this.convertValues(source["parts"], Part);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace config {
	
	export class App {
	    gameRoot: string;
	    modsRepo: string;
	    updateUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new App(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameRoot = source["gameRoot"];
	        this.modsRepo = source["modsRepo"];
	        this.updateUrl = source["updateUrl"];
	    }
	}
	export class LogEntry {
	    time: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.message = source["message"];
	    }
	}
	export class SubModInfo {
	    name: string;
	    parts?: Record<string, string[]>;
	    cover?: string;
	    previews?: string[];
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SubModInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.parts = source["parts"];
	        this.cover = source["cover"];
	        this.previews = source["previews"];
	        this.enabled = source["enabled"];
	    }
	}
	export class ModInfo {
	    name: string;
	    nickname: string;
	    path: string;
	    cover: string;
	    previews?: string[];
	    enabled: boolean;
	    installed?: boolean;
	    parts?: Record<string, string[]>;
	    missing?: boolean;
	    category?: string;
	    submods?: SubModInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ModInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.nickname = source["nickname"];
	        this.path = source["path"];
	        this.cover = source["cover"];
	        this.previews = source["previews"];
	        this.enabled = source["enabled"];
	        this.installed = source["installed"];
	        this.parts = source["parts"];
	        this.missing = source["missing"];
	        this.category = source["category"];
	        this.submods = this.convertValues(source["submods"], SubModInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class AboutInfo {
	    label: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new AboutInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.value = source["value"];
	    }
	}
	export class PendingItem {
	    mod: string;
	    subMod: string;
	    slot: string;
	    chinese: string;
	    english: string;
	
	    static createFrom(source: any = {}) {
	        return new PendingItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mod = source["mod"];
	        this.subMod = source["subMod"];
	        this.slot = source["slot"];
	        this.chinese = source["chinese"];
	        this.english = source["english"];
	    }
	}
	export class SubModConfig {
	    name: string;
	    parts: Record<string, string[]>;
	    cover?: string;
	    previews?: string[];
	
	    static createFrom(source: any = {}) {
	        return new SubModConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.parts = source["parts"];
	        this.cover = source["cover"];
	        this.previews = source["previews"];
	    }
	}
	export class modConfig {
	    nickname: string;
	    category: string;
	    cover: string;
	    previews?: string[];
	    parts: Record<string, string[]>;
	    submods?: SubModConfig[];
	
	    static createFrom(source: any = {}) {
	        return new modConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nickname = source["nickname"];
	        this.category = source["category"];
	        this.cover = source["cover"];
	        this.previews = source["previews"];
	        this.parts = source["parts"];
	        this.submods = this.convertValues(source["submods"], SubModConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BatchGenerateResult {
	    total: number;
	    generated: number;
	    mods: modConfig[];
	    pending: PendingItem[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new BatchGenerateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.generated = source["generated"];
	        this.mods = this.convertValues(source["mods"], modConfig);
	        this.pending = this.convertValues(source["pending"], PendingItem);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EngineFile {
	    name: string;
	    isDir: boolean;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EngineFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.isDir = source["isDir"];
	        this.exists = source["exists"];
	    }
	}
	export class EngineStatus {
	    gameRoot: string;
	    present: boolean;
	    files: EngineFile[];
	
	    static createFrom(source: any = {}) {
	        return new EngineStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameRoot = source["gameRoot"];
	        this.present = source["present"];
	        this.files = this.convertValues(source["files"], EngineFile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportResult {
	    name: string;
	    nickname: string;
	    category: string;
	    cover: string;
	    previews?: string[];
	    parts: Record<string, string[]>;
	    submods?: SubModConfig[];
	    configFound: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.nickname = source["nickname"];
	        this.category = source["category"];
	        this.cover = source["cover"];
	        this.previews = source["previews"];
	        this.parts = source["parts"];
	        this.submods = this.convertValues(source["submods"], SubModConfig);
	        this.configFound = source["configFound"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModCard {
	    nickname: string;
	    category: string;
	    cover: string;
	    previews?: string[];
	    parts: Record<string, string[]>;
	
	    static createFrom(source: any = {}) {
	        return new ModCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nickname = source["nickname"];
	        this.category = source["category"];
	        this.cover = source["cover"];
	        this.previews = source["previews"];
	        this.parts = source["parts"];
	    }
	}
	export class ModConflict {
	    modName: string;
	    nickname: string;
	    slot: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new ModConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modName = source["modName"];
	        this.nickname = source["nickname"];
	        this.slot = source["slot"];
	        this.value = source["value"];
	    }
	}
	export class ModConflictInfo {
	    modName: string;
	    nickname: string;
	    conflicts: ModConflict[];
	
	    static createFrom(source: any = {}) {
	        return new ModConflictInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modName = source["modName"];
	        this.nickname = source["nickname"];
	        this.conflicts = this.convertValues(source["conflicts"], ModConflict);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    downloadUrl: string;
	    notes: string;
	    hasUpdate: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.downloadUrl = source["downloadUrl"];
	        this.notes = source["notes"];
	        this.hasUpdate = source["hasUpdate"];
	        this.message = source["message"];
	    }
	}

}

