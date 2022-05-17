import React, {useEffect, useState} from 'react';
import {Link} from '@nextui-org/react';
import {ReportEntry, ReportIndex} from '../types';

function fetchIndex(basePath: string, setter: (data: any) => void) {
  return fetch(
    `${basePath}/index.json`,
  )
    .then((res) => res.json())
    .then((data) => {
      console.log("reportIndex", data);
      setter(data);
    })
    .catch((e) => {
      console.error("failed to fetch index", e)
    });
}

interface ReportNavigatorProps {
  onSelect: (reportEntry: ReportEntry) => void;
}

const ReportNavigator = (props: ReportNavigatorProps) => {
  const [isLoading, setLoading] = useState(false)
  const [reportIndex, setReportIndex] = useState<ReportIndex>({runs: []});

  useEffect(() => {
    setLoading(true)
    fetchIndex('/output', setReportIndex).then(() => {
      setLoading(false);
    })
  }, []);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (reportIndex.runs.length == 0) {
    return <div>No back-test report data</div>
  }

  return <div>
    {
      reportIndex.runs.map((entry) => {
        return <Link key={entry.id} onClick={() => {
          if (props.onSelect) {
            props.onSelect(entry);
          }
        }}>{entry.id}</Link>
      })
    }
  </div>;


};

export default ReportNavigator;
