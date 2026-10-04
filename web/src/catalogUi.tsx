import { useEffect, type ReactNode } from 'react';
import { Link, useNavigate } from 'react-router';
import { useQueryClient } from '@tanstack/react-query';
import { isUnauthenticated, type Item } from './api';
import { returnStatus } from './uiModel';
import styles from './styles.module.css';

export function NavIcon({ name }: { name: 'items' | 'locations' | 'categories' | 'returns' | 'account' }) {
  const paths = {
    items: <><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4 4"/></>,
    locations: <><path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="M3 8v9l9 5 9-5V8M12 13v9"/></>,
    categories: <><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/></>,
    returns: <><path d="M4 10a8 8 0 1 1 1 8M4 4v6h6"/><path d="M12 7v5l3 2"/></>,
    account: <><circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/></>,
  };
  return <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

export function PageHeading({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return <div className={styles.pageHeading}><div><h1 className={styles.title}>{title}</h1>{description ? <p className={styles.intro}>{description}</p> : null}</div>{action}</div>;
}

export function QueryError({ error, retry, pending }: { error: unknown; retry: () => void; pending?: boolean }) {
  if (isUnauthenticated(error)) return <SessionExpired/>;
  return <div className={styles.inlineNotice} role="alert"><p>{error instanceof Error ? error.message : '暂时无法读取，请重试'}</p><button className={styles.button} onClick={retry} disabled={pending}>重试</button></div>;
}

export function SessionExpired() {
  const client=useQueryClient();
  const navigate=useNavigate();
  useEffect(()=>{client.setQueryData(['me'],null);navigate('/login',{replace:true});},[client,navigate]);
  return <p className={styles.meta}>登录已过期，正在返回登录页…</p>;
}

export function ItemSummary({ item }: { item: Item }) {
  const status = returnStatus(item.return_tasks);
  const tasks = item.return_tasks.filter(task => !task.completed_at);
  const cover = item.photos[0];
  return <>
    {cover ? <img className={styles.itemThumb} src={`/api/v1/photos/${cover.id}/thumbnail`} alt="" loading="lazy"/> : <span className={styles.itemPlaceholder}><NavIcon name="locations"/></span>}
    <span className={styles.itemLinkBody}>
      <span className={styles.itemName}>{item.name}{status ? <span className={styles.statusBadge}>{status}</span> : null}</span>
      {item.model ? <span className={styles.itemMeta}>{item.model}</span> : null}
      {tasks.map(task => <span className={styles.itemMeta} key={task.id}>{task.part_note ? `${task.part_note} · ` : ''}{[task.reason, task.destination_note].filter(Boolean).join(' · ') || '临时取出，等待归位'}</span>)}
      {item.locations.length ? item.locations.map(loc => <span key={loc.location_id} className={styles.path}>{tasks.length ? '归位：' : ''}{loc.path.map(n => `${n.name}${n.code ? ` ${n.code}` : ''}`).join(' / ')}{loc.note ? ` · ${loc.note}` : ''}</span>) : <span className={styles.itemMeta}>尚未记录位置</span>}
    </span><span aria-hidden="true">›</span>
  </>;
}

export function ItemResult({ item, from }: { item: Item; from?: string }) {
  return <li><Link className={styles.itemLink} to={`/items/${item.id}`} state={from ? { from } : undefined}><ItemSummary item={item}/></Link></li>;
}
