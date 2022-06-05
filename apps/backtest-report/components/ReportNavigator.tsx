import React, {useEffect, useState} from 'react';
import {List, ThemeIcon} from '@mantine/core';
import {CircleCheck} from 'tabler-icons-react';

import {ReportEntry, ReportIndex} from '../types';

function fetchIndex(basePath: string, setter: (data: any) => void) {
  return fetch(
    `${basePath}/index.json`,
  )
    .then((res) => res.json())
    .then((data) => {
      console.log("reportIndex", data);
      data.runs.reverse() // last reports render first
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

  return <div className={"report-navigator"}>
    <List
      spacing="xs"
      size="xs"
      center
      icon={
        <ThemeIcon color="teal" size={16} radius="xl">
          <CircleCheck size={16}/>
        </ThemeIcon>
      }
    >
      {
        reportIndex.runs.map((entry) => {
          return <List.Item key={entry.id} onClick={() => {
            if (props.onSelect) {
              props.onSelect(entry);
            }
          }}>
            <div style={{
              "textOverflow": "ellipsis",
              "overflow": "hidden",
              "inlineSize": "190px",
            }}>
            {entry.id}
            </div>
          </List.Item>
        })
      }
    </List>
  </div>;


};

export default ReportNavigator;
