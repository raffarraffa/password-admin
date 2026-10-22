export namespace main {
	
	export class Account {
	    id: number;
	    name: string;
	    passwordCount: number;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.passwordCount = source["passwordCount"];
	    }
	}
	export class GitHubConfig {
	    owner: string;
	    repo: string;
	    initialized?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GitHubConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.repo = source["repo"];
	        this.initialized = source["initialized"];
	    }
	}
	export class ImportResult {
	    accountName: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountName = source["accountName"];
	        this.count = source["count"];
	    }
	}
	export class PasswordEntry {
	    id: number;
	    name: string;
	    url: string;
	    username: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new PasswordEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class PasswordSummary {
	    id: number;
	    name: string;
	    url: string;
	    username: string;
	    accountName: string;
	
	    static createFrom(source: any = {}) {
	        return new PasswordSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.username = source["username"];
	        this.accountName = source["accountName"];
	    }
	}
	export class VaultStatus {
	    configured: boolean;
	    hasToken: boolean;
	    initialized: boolean;
	    unlocked: boolean;
	    remoteExists: boolean;
	    config: GitHubConfig;
	
	    static createFrom(source: any = {}) {
	        return new VaultStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.hasToken = source["hasToken"];
	        this.initialized = source["initialized"];
	        this.unlocked = source["unlocked"];
	        this.remoteExists = source["remoteExists"];
	        this.config = this.convertValues(source["config"], GitHubConfig);
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

