export namespace main {
	
	export class FileCard {
	    id: number;
	    name: string;
	    size: number;
	    sha256: string;
	    chunkCount: number;
	    savedBytes: number;
	    savedPercent: number;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new FileCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	        this.chunkCount = source["chunkCount"];
	        this.savedBytes = source["savedBytes"];
	        this.savedPercent = source["savedPercent"];
	        this.createdAt = source["createdAt"];
	    }
	}

}

