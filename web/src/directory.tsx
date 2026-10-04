import { useQuery } from '@tanstack/react-query';
import { useState, type CSSProperties } from 'react';
import { Link, useSearchParams } from 'react-router';
import { fetchAllCategories, fetchAllLocations, listLocationIcons } from './api';
import { LocationGlyph } from './icons';
import { NavIcon, PageHeading, QueryError } from './catalogUi';
import { locationDirectoryRows, searchLocations, treeRows } from './uiModel';
import styles from './styles.module.css';

export function LocationDirectoryPage() { return <Directory kind="locations"/>; }
export function CategoryDirectoryPage() { return <Directory kind="categories"/>; }

function Directory({kind}:{kind:'locations'|'categories'}) {
  const isLocation = kind==='locations';
  const [params,setParams] = useSearchParams();
  const q = params.get('q') ?? '';
  const types = [{value:'all',label:'全部'},{value:'area',label:'区域'},{value:'fixed',label:'柜格'},{value:'movable',label:'盒子与容器'}];
  const type = isLocation && types.some(row=>row.value===params.get('type')) ? params.get('type')! : 'all';
  const flat = Boolean(q.trim()) || type !== 'all';
  function setCondition(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value && !(key === 'type' && value === 'all')) next.set(key,value); else next.delete(key);
    setParams(next,{replace:true});
  }
  const [expanded,setExpanded] = useState(new Set<number>());
  const locations = useQuery({queryKey:['locations','flat'],queryFn:()=>fetchAllLocations(new URLSearchParams({flat:'1'})),enabled:isLocation,retry:false});
  const categories = useQuery({queryKey:['categories','flat'],queryFn:()=>fetchAllCategories(new URLSearchParams({flat:'1'})),enabled:!isLocation,retry:false});
  const icons=useQuery({queryKey:['location-icons'],queryFn:listLocationIcons,enabled:isLocation,retry:false});
  const query = isLocation ? locations : categories;
  const nodes: {id:number;name:string;parent_id:number|null;code:string|null;direct_item_count:number;path:{name:string;code:string|null}[]}[] = isLocation ? locations.data ?? [] : (categories.data ?? []).map(n=>({...n,code:null,path:n.path.map(p=>({...p,code:null}))}));
  const rows = isLocation ? locationDirectoryRows(locations.data ?? [],expanded,q,type) : q.trim() ? searchLocations(q,nodes).map(node=>({node,depth:0,hasChildren:false})) : treeRows(nodes,expanded);
  const locationMap = new Map((locations.data ?? []).map(node=>[node.id,node]));
  const label = isLocation ? '位置' : '分类';
  return <>
    <PageHeading title={isLocation ? '每件东西，都有去处' : '按分类，慢慢找'} description={isLocation ? '从房间到柜格，再到收纳盒。' : '不记得名字，也能找到家里已有的东西。'} action={<Link className={styles.buttonPrimary} to={`/${kind}/new`}>＋ 新增{label}</Link>}/>
    <div className={styles.searchRow}><NavIcon name={kind}/><input type="search" aria-label={`搜索${label}`} placeholder={isLocation ? '搜索位置名称或编号，例如 B101' : '搜索分类名称'} value={q} onChange={e=>setCondition('q',e.target.value)}/></div>
    {isLocation ? <div className={styles.chips} aria-label="位置类型">{types.map(row=><button key={row.value} className={type===row.value ? styles.chipActive : styles.chip} aria-pressed={type===row.value} onClick={()=>setCondition('type',row.value)}>{row.label}<span className={styles.countMuted}>{row.value==='all' ? nodes.length : locations.data?.filter(node=>node.type===row.value).length ?? 0}</span></button>)}</div> : null}
    <div className={styles.resultsHeader}><p className={styles.resultCount}>{flat ? `${rows.length} 个匹配${label}` : `全屋 ${nodes.length} 个${label}`}</p>{!flat ? <button className={styles.button} onClick={()=>setExpanded(expanded.size ? new Set() : new Set(nodes.map(n=>n.id)))}>{expanded.size ? '收起全部' : '展开全部'}</button> : <button className={styles.button} onClick={()=>setParams({},{replace:true})}>清除条件</button>}</div>
    {query.isPending ? <p className={styles.meta}>正在读取{label}…</p> : null}
    {query.isError ? <QueryError error={query.error} retry={()=>void query.refetch()} pending={query.isFetching}/> : null}
    {!query.isPending && !query.isError && !rows.length ? <div className={styles.emptyState}><NavIcon name={kind}/><h2>{flat ? `没有符合条件的${label}` : `从第一个${label}开始`}</h2><p>{flat ? '试试名称的一部分，或清除条件浏览全部。' : isLocation ? '先建房间，再添加柜格和收纳盒。' : '分类按需要慢慢建立，不必一次想齐。'}</p></div> : null}
    <ul className={styles.list}>
      {rows.map(({node,depth,hasChildren})=><li key={node.id} className={styles.treeRow} style={{'--tree-depth':Math.min(depth,5)} as CSSProperties}>
        {hasChildren ? <button className={styles.treeToggle} aria-label={`${expanded.has(node.id) ? '收起' : '展开'}${node.name}`} aria-expanded={expanded.has(node.id)} onClick={()=>setExpanded(prev=>{const next=new Set(prev);if(next.has(node.id))next.delete(node.id);else next.add(node.id);return next;})}>{expanded.has(node.id) ? '⌄' : '›'}</button> : <span className={styles.treeToggle}/>}
        <Link className={styles.itemLink} state={{from:`/${kind}${params.size ? `?${params}` : ''}`}} to={`/${kind}/${node.id}`}>{isLocation && locationMap.has(node.id) ? <LocationGlyph node={locationMap.get(node.id)!} library={icons.data ?? []} className={styles.rowIcon}/> : <NavIcon name={kind}/>}<span className={styles.itemLinkBody}><span className={styles.itemName}>{node.name}{node.code ? <span className={styles.codeBadge}>{node.code}</span> : null}</span>{flat ? <span className={styles.path}>{node.path.map(n=>n.name).join(' / ')}</span> : null}<span className={styles.itemMeta}>{isLocation ? '查看这里的物品与下级位置' : '浏览这一类物品'}</span></span><span>›</span></Link>
      </li>)}
    </ul>
    {isLocation ? <Link className={styles.subtleLink} to="/locations/manage">管理空位置与批量删除</Link> : null}
  </>;
}
