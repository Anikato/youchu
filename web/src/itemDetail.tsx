import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link, useLocation, useParams } from 'react-router';
import { completeReturnTask, createReturnTask, getItem, isNotFound, isUnauthenticated, type ReturnTask } from './api';
import { NavIcon, PageHeading, QueryError, SessionExpired } from './catalogUi';
import { returnStatus } from './uiModel';
import styles from './styles.module.css';

export function ItemDetailPage() {
  const id = useParams().id ?? '';
  return <ItemDetail key={id} id={id}/>;
}

function ItemDetail({id}: {id:string}) {
  const client = useQueryClient();
  const location = useLocation();
  const rawFrom = (location.state as {from?:unknown} | null)?.from;
  const from = typeof rawFrom === 'string' && rawFrom.startsWith('/') && !rawFrom.startsWith('//') ? rawFrom : '/';
  const query = useQuery({queryKey:['item',id],queryFn:()=>getItem(id),retry:false});
  const [photoIndex,setPhotoIndex] = useState(0);
  const [adding,setAdding] = useState(false);
  const [destination,setDestination] = useState('');
  const [reason,setReason] = useState('借出');
  const [part,setPart] = useState('');
  const [notice,setNotice] = useState('');
  async function refresh() {
    await Promise.all([client.invalidateQueries({queryKey:['item',id]}),client.invalidateQueries({queryKey:['items']}),client.invalidateQueries({queryKey:['return-tasks']})]);
  }
  const complete = useMutation({mutationFn:(task:ReturnTask)=>completeReturnTask(task.id,task.version),onSuccess:async()=>{setNotice('已完成归位');await refresh();},onError:()=>void refresh()});
  const add = useMutation({mutationFn:()=>createReturnTask(id,{reason,destination_note:destination.trim() || null,part_note:part.trim() || null}),onSuccess:async()=>{setAdding(false);setDestination('');setPart('');setNotice('已记下，归位前会持续保留');await refresh();}});
  if (isUnauthenticated(query.error) || isUnauthenticated(add.error) || isUnauthenticated(complete.error)) return <SessionExpired/>;
  if (query.isPending) return <p className={styles.meta}>正在读取物品…</p>;
  if (isNotFound(query.error)) return <div className={styles.emptyState}><h1>这件物品已不存在</h1><Link to="/">回到查找</Link></div>;
  if (query.isError) return <QueryError error={query.error} retry={()=>void query.refetch()} pending={query.isFetching}/>;
  const item = query.data;
  const tasks = item.return_tasks.filter(t=>!t.completed_at);
  const status = returnStatus(item.return_tasks);
  const photo = item.photos[photoIndex] ?? item.photos[0];
  const info = [['别名',item.alias],['型号',item.model],['规格',item.spec],['数量',item.quantity_note],['备注',item.note]].filter(([,value])=>value);
  return <>
    <Link className={styles.subtleLink} to={from}>← 返回目录</Link>
    <PageHeading title={item.name} description={item.model ?? undefined} action={<Link className={styles.button} to={`/items/${id}/edit`} state={{from}}>编辑资料</Link>}/>
    {notice ? <p className={styles.inlineNotice} role="status">{notice}</p> : null}
    {tasks.length ? <section className={styles.uploadNotice}>
      <h2 className={styles.sectionTitle}>{status}</h2>
      {tasks.map(task=><div className={styles.infoRow} key={task.id}><div><strong>{task.part_note || '整件物品'}{task.reason ? ` · ${task.reason}` : ''}</strong><p>{task.destination_note || '尚未填写临时去向'}</p></div><button className={styles.button} disabled={complete.isPending} onClick={()=>complete.mutate(task)}>完成归位</button></div>)}
      {complete.error ? <p className={styles.error} role="alert">{complete.error.message}</p> : null}
    </section> : null}
    <div className={styles.detailGrid}>
      <section className={styles.detailHero}>
        {photo ? <a href={`/api/v1/photos/${photo.id}/original`} target="_blank" rel="noreferrer"><img className={styles.detailPhoto} src={`/api/v1/photos/${photo.id}/original`} alt={item.name}/></a> : <div className={styles.emptyState}><NavIcon name="locations"/><p>用名称和位置找到它</p><Link className={styles.subtleLink} to={`/items/${id}/edit`} state={{from}}>添加辨认照片</Link></div>}
        {item.photos.length > 1 ? <div className={styles.gallery}>{item.photos.map((p,index)=><button className={index===photoIndex ? styles.chipActive : styles.chip} key={p.id} onClick={()=>setPhotoIndex(index)} aria-label={`查看第 ${index+1} 张照片`} aria-pressed={index===photoIndex}><img src={`/api/v1/photos/${p.id}/thumbnail`} alt=""/></button>)}</div> : null}
      </section>
      <section className={styles.detailInfo}>
        <p className={styles.eyebrow}>{tasks.length ? '正式归位位置' : '存放在哪里'}</p>
        <h2 className={styles.sectionTitle}>{item.locations.length ? `${item.locations.length} 个存放位置` : '等待定位'}</h2>
        {item.locations.map(loc=><Link key={loc.location_id} className={styles.locationCard} to={`/locations/${loc.location_id}`}><NavIcon name="locations"/><span><strong>{loc.path.at(-1)?.name}</strong>{loc.path.at(-1)?.code ? <span className={styles.codeBadge}>{loc.path.at(-1)?.code}</span> : null}<span className={styles.path}>{loc.path.map(n=>n.name).join(' / ')}</span>{loc.note ? <span className={styles.itemMeta}>{loc.note}</span> : null}</span><span>›</span></Link>)}
        {!item.locations.length ? <p className={styles.meta}>先记下来也没关系，整理时再补上位置。</p> : null}
        <div className={styles.detailActions}><Link className={styles.button} to={`/items/${id}/edit`} state={{from}}>调整位置</Link><button className={styles.button} onClick={()=>setAdding(!adding)} aria-expanded={adding}>{adding ? '取消登记' : '借出 / 临时取出'}</button></div>
        {adding ? <form className={styles.editor} onSubmit={e=>{e.preventDefault();add.mutate();}}>
          <label className={styles.field}>情况<select value={reason} onChange={e=>setReason(e.target.value)}><option>借出</option><option>带出门</option><option>临时取出</option></select></label>
          <label className={styles.field}>临时去向<input value={destination} onChange={e=>setDestination(e.target.value)} placeholder="例如：借给谁、现在放在哪"/></label>
          <label className={styles.field}>取出的配件（整件留空）<input value={part} onChange={e=>setPart(e.target.value)}/></label>
          {add.error ? <p className={styles.error} role="alert">{add.error.message}</p> : null}
          <button className={styles.buttonPrimary} disabled={add.isPending}>记下待归位</button>
        </form> : null}
      </section>
    </div>
    <section>
      <h2 className={styles.sectionTitle}>物品资料</h2>
      {info.length ? <dl>{info.map(([label,value])=><div className={styles.infoRow} key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl> : <p className={styles.meta}>没有额外说明</p>}
      <div className={styles.chips}>{item.categories.map(cat=><Link className={styles.chip} key={cat.category_id} to={`/categories/${cat.category_id}`}>{cat.path.map(n=>n.name).join(' / ')}</Link>)}</div>
    </section>
  </>;
}
