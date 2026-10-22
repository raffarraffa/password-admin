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
	    initialized: boolean;
	    unlocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VaultStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.initialized = source["initialized"];
	        this.unlocked = source["unlocked"];
	    }
	}

}

