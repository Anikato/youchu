// Keep failed files in this tab while the user signs in again; never persist photos to localStorage.
export const pendingUpload: {current: {id:number;name:string;files:File[]} | null} = {current:null};

export async function uploadBatch<T>(files: T[], upload: (file: T, index: number) => Promise<unknown>): Promise<{file:T;error:unknown}[]> {
  const failed: {file:T;error:unknown}[] = [];
  for (let i=0;i<files.length;i++) {
    try { await upload(files[i],i); }
    catch(error) { failed.push({file:files[i],error}); }
  }
  return failed;
}
